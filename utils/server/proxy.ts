import dns from "node:dns";
import { BlockList, isIP } from "node:net";
import { Agent, buildConnector, fetch } from "undici";

// The proxy fetches whatever URL a visitor names, so on a public deployment it
// must not reach the host's own network: loopback, private ranges, link-local
// (cloud metadata lives at 169.254.169.254) and the like are refused. The check
// runs at connect time, on the address actually dialled, so DNS rebinding and
// redirects into a private range are caught too.

// One list per family: BlockList also matches IPv4 against IPv6 rules
// (as ::a.b.c.d / ::ffff:a.b.c.d), so ::/96 in a shared list would block
// every IPv4 address.
const blocked4 = new BlockList();
const blocked6 = new BlockList();
for (const [net, prefix] of [
  ["0.0.0.0", 8], ["10.0.0.0", 8], ["100.64.0.0", 10], ["127.0.0.0", 8],
  ["169.254.0.0", 16], ["172.16.0.0", 12], ["192.0.0.0", 24], ["192.0.2.0", 24],
  ["192.88.99.0", 24], ["192.168.0.0", 16], ["198.18.0.0", 15],
  ["198.51.100.0", 24], ["203.0.113.0", 24], ["224.0.0.0", 4], ["240.0.0.0", 4],
] as const) blocked4.addSubnet(net, prefix, "ipv4");
for (const [net, prefix] of [
  ["::", 96],          // unspecified, loopback, IPv4-compatible
  ["::ffff:0:0", 96],  // IPv4-mapped: would smuggle any IPv4 address
  ["64:ff9b::", 96], ["64:ff9b:1::", 48], // NAT64
  ["100::", 64], ["2001::", 32], ["2001:db8::", 32], ["2002::", 16],
  ["fc00::", 7], ["fe80::", 10], ["ff00::", 8],
] as const) blocked6.addSubnet(net, prefix, "ipv6");

export function isBlockedAddress(ip: string): boolean {
  const family = isIP(ip);
  if (family === 0) return true;
  return family === 4 ? blocked4.check(ip, "ipv4") : blocked6.check(ip, "ipv6");
}

export class ProxyError extends Error {
  constructor(message: string, readonly httpStatus: number, readonly statusText: string) {
    super(message);
  }
}

const blockedError = (host: string) =>
  new ProxyError(
    `${host} is a private or reserved address, which this proxy does not reach. ` +
      "Localhost goes through the browser extension; for other internal hosts, self-host with PROXY_ALLOW_PRIVATE_NETWORKS=true.",
    403,
    "Blocked",
  );

type LookupCb = (err: Error | null, address: string | dns.LookupAddress[], family?: number) => void;

// An undici agent that refuses to connect to any address isBlocked flags.
export function createSafeAgent(isBlocked: (ip: string) => boolean = isBlockedAddress): Agent {
  const lookup = (hostname: string, options: dns.LookupOptions, cb: LookupCb) => {
    dns.lookup(hostname, { ...options, all: true }, (err, addresses) => {
      if (err) return cb(err, "");
      // Every record must be public: a mixed answer could be raced to the private one.
      if (addresses.length === 0 || addresses.some((a) => isBlocked(a.address))) {
        return cb(blockedError(hostname), "");
      }
      if (options.all) cb(null, addresses);
      else cb(null, addresses[0].address, addresses[0].family);
    });
  };
  const connect = buildConnector({ lookup } as buildConnector.BuildOptions);
  return new Agent({
    connect(opts, cb) {
      // net.connect skips lookup for IP literals, so check those here.
      const host = opts.hostname.replace(/^\[|\]$/g, "");
      if (isIP(host) && isBlocked(host)) return cb(blockedError(host), null);
      connect(opts, cb);
    },
  });
}

const safeAgent = createSafeAgent();

const num = (v: string | undefined, fallback: number) => {
  const n = Number(v);
  return v !== undefined && v !== "" && Number.isFinite(n) && n >= 0 ? n : fallback;
};

export function proxyConfig() {
  return {
    allowPrivate: process.env.PROXY_ALLOW_PRIVATE_NETWORKS === "true",
    timeoutMs: num(process.env.PROXY_TIMEOUT_MS, 30_000),
    // Vercel caps a function response at 4.5 MB, and the JSON envelope adds some.
    maxResponseBytes: num(process.env.PROXY_MAX_RESPONSE_BYTES, 4 * 1024 * 1024),
    maxRequestBytes: num(process.env.PROXY_MAX_REQUEST_BYTES, 4 * 1024 * 1024),
    rateLimitPerMinute: num(process.env.PROXY_RATE_LIMIT_PER_MINUTE, 120),
  };
}

// ponytail: per-instance memory, so the limit is per serverless instance, not
// global. Good enough to stop one client hammering one instance; put a Vercel
// Firewall rate-limit rule (or Redis) in front for a hard global limit.
const hits = new Map<string, { count: number; resetAt: number }>();

export function rateLimit(key: string, limit: number, now = Date.now()): { ok: true } | { ok: false; retryAfter: number } {
  if (limit <= 0) return { ok: true };
  if (hits.size > 10_000) for (const [k, v] of hits) if (v.resetAt <= now) hits.delete(k);
  const entry = hits.get(key);
  if (!entry || entry.resetAt <= now) {
    hits.set(key, { count: 1, resetAt: now + 60_000 });
    return { ok: true };
  }
  if (entry.count >= limit) return { ok: false, retryAfter: Math.ceil((entry.resetAt - now) / 1000) };
  entry.count++;
  return { ok: true };
}

export function resetRateLimit() {
  hits.clear();
}

const HOP_BY_HOP = new Set([
  "host", "content-length", "transfer-encoding", "connection", "keep-alive",
  "proxy-connection", "proxy-authorization", "upgrade", "te", "trailer",
]);
const FORBIDDEN_METHODS = new Set(["CONNECT", "TRACE", "TRACK"]);

export function cleanHeaders(input: unknown): Record<string, string> {
  let raw = input;
  if (typeof raw === "string") {
    try {
      raw = JSON.parse(raw || "{}");
    } catch {
      throw new ProxyError("Headers must be a JSON object", 400, "Bad Request");
    }
  }
  if (raw === null || raw === undefined) return {};
  if (typeof raw !== "object" || Array.isArray(raw)) {
    throw new ProxyError("Headers must be a JSON object", 400, "Bad Request");
  }
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(raw as Record<string, unknown>)) {
    if (!k.trim() || HOP_BY_HOP.has(k.toLowerCase()) || v === undefined || v === null) continue;
    out[k] = String(v);
  }
  return out;
}

export function parseSetCookies(lines: string[]): Record<string, string> {
  const cookies: Record<string, string> = {};
  for (const line of lines) {
    const pair = line.split(";")[0];
    const eq = pair.indexOf("=");
    if (eq > 0) cookies[pair.slice(0, eq).trim()] = pair.slice(eq + 1).trim();
  }
  return cookies;
}

export type ProxyRequest = { url: unknown; method: unknown; headers?: unknown; body?: unknown };

export type ProxyResult = {
  data: unknown;
  status: number;
  statusText: string;
  ok: boolean;
  headers: Record<string, string>;
  cookies: Record<string, string>;
  error?: string;
};

export async function forward(req: ProxyRequest, opts: { dispatcher?: Agent } = {}): Promise<ProxyResult> {
  const cfg = proxyConfig();

  if (typeof req.url !== "string" || !req.url || typeof req.method !== "string" || !req.method) {
    throw new ProxyError("Missing required fields: url, method", 400, "Bad Request");
  }
  let target: URL;
  try {
    target = new URL(req.url);
  } catch {
    throw new ProxyError(`Invalid URL: ${req.url}`, 400, "Bad Request");
  }
  if (target.protocol !== "http:" && target.protocol !== "https:") {
    throw new ProxyError("Only http and https URLs can be proxied", 400, "Bad Request");
  }
  const method = req.method.toUpperCase();
  if (!/^[A-Z]+$/.test(method) || FORBIDDEN_METHODS.has(method)) {
    throw new ProxyError(`Method ${req.method} is not allowed`, 400, "Bad Request");
  }
  const headers = cleanHeaders(req.headers);

  let body: string | undefined;
  if (method !== "GET" && method !== "HEAD") {
    if (typeof req.body === "string") body = req.body;
    else if (req.body !== undefined && req.body !== null) body = JSON.stringify(req.body);
  }

  let upstream: Awaited<ReturnType<typeof fetch>>;
  try {
    upstream = await fetch(target, {
      method,
      headers,
      body,
      redirect: "follow",
      signal: AbortSignal.timeout(cfg.timeoutMs),
      dispatcher: opts.dispatcher ?? (cfg.allowPrivate ? undefined : safeAgent),
    });
  } catch (err) {
    throw toProxyError(err, cfg.timeoutMs);
  }

  const responseHeaders: Record<string, string> = {};
  upstream.headers.forEach((value, key) => {
    if (key !== "set-cookie") responseHeaders[key] = value;
  });
  const setCookies = upstream.headers.getSetCookie();
  if (setCookies.length) responseHeaders["set-cookie"] = setCookies.join("\n");

  const noBody = method === "HEAD" || [101, 103, 204, 205, 304].includes(upstream.status);
  let data: unknown = null;
  if (!noBody && upstream.body) {
    const raw = await readCapped(upstream, cfg.maxResponseBytes, cfg.timeoutMs);
    if (raw) {
      const type = upstream.headers.get("content-type") ?? "";
      data = raw;
      if (/[/+]json\b/i.test(type)) {
        try {
          data = JSON.parse(raw);
        } catch {
          // not valid JSON after all: show it as text
        }
      }
    }
  }

  return {
    data: method === "HEAD" ? responseHeaders : data,
    status: upstream.status,
    statusText: upstream.statusText,
    ok: upstream.ok,
    headers: responseHeaders,
    cookies: parseSetCookies(setCookies),
  };
}

async function readCapped(res: Awaited<ReturnType<typeof fetch>>, max: number, timeoutMs: number): Promise<string> {
  const tooLarge = () =>
    new ProxyError(`Response is larger than the proxy limit of ${max} bytes`, 502, "Response Too Large");
  const declared = Number(res.headers.get("content-length"));
  if (Number.isFinite(declared) && declared > max) {
    await res.body?.cancel();
    throw tooLarge();
  }
  const reader = res.body!.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > max) {
        await reader.cancel();
        throw tooLarge();
      }
      chunks.push(value);
    }
  } catch (err) {
    throw toProxyError(err, timeoutMs);
  }
  return Buffer.concat(chunks).toString("utf8");
}

function toProxyError(err: unknown, timeoutMs: number): ProxyError {
  if (err instanceof ProxyError) return err;
  const e = err as { name?: string; message?: string; cause?: unknown };
  const cause = e?.cause as { name?: string; message?: string; code?: string } | undefined;
  if (cause instanceof ProxyError) return cause;
  if (err instanceof TypeError && !cause && /header|method/i.test(err.message)) {
    return new ProxyError(err.message, 400, "Bad Request");
  }
  if (e?.name === "TimeoutError" || cause?.name === "TimeoutError" || e?.name === "AbortError") {
    return new ProxyError(`Upstream did not answer within ${timeoutMs} ms`, 504, "Timeout");
  }
  const detail = cause?.code ? `${cause.code}: ${cause.message}` : cause?.message || e?.message;
  return new ProxyError(detail || "Failed to connect to target URL", 502, "Network Error");
}
