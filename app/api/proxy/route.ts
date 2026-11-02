import { NextResponse, NextRequest } from "next/server";
import { ProxyError, forward, proxyConfig, rateLimit, type ProxyRequest } from "@/utils/server/proxy";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// Every reply, error or not, has the same shape, so the client can always render it.
function fail(err: ProxyError, extraHeaders?: Record<string, string>) {
  return NextResponse.json(
    { data: null, status: 0, statusText: err.statusText, ok: false, headers: {}, cookies: {}, error: err.message },
    { status: err.httpStatus, headers: extraHeaders },
  );
}

function clientKey(req: NextRequest): string {
  // Vercel and most reverse proxies set these; the first hop is the client.
  return (
    req.headers.get("x-real-ip") ||
    req.headers.get("x-forwarded-for")?.split(",")[0].trim() ||
    "unknown"
  );
}

export async function POST(req: NextRequest) {
  const cfg = proxyConfig();

  const limited = rateLimit(clientKey(req), cfg.rateLimitPerMinute);
  if (!limited.ok) {
    return fail(new ProxyError(`Too many requests: the limit is ${cfg.rateLimitPerMinute} a minute`, 429, "Too Many Requests"), {
      "Retry-After": String(limited.retryAfter),
    });
  }

  const declared = Number(req.headers.get("content-length"));
  if (Number.isFinite(declared) && declared > cfg.maxRequestBytes) {
    return fail(new ProxyError(`Request is larger than the proxy limit of ${cfg.maxRequestBytes} bytes`, 413, "Payload Too Large"));
  }

  const tooLarge = () => fail(new ProxyError(`Request is larger than the proxy limit of ${cfg.maxRequestBytes} bytes`, 413, "Payload Too Large"));

  let parsed: ProxyRequest;
  // A file upload: the request as JSON in a `request` part, its body bytes in a `body` part.
  if ((req.headers.get("content-type") ?? "").toLowerCase().startsWith("multipart/form-data")) {
    // A plain HTML form can post multipart cross-site without a preflight; this header can't be set by one.
    if (req.headers.get("x-hittable") !== "1") return fail(new ProxyError("Missing X-Hittable header", 403, "Forbidden"));
    try {
      const form = await req.formData();
      const meta = JSON.parse(String(form.get("request") ?? ""));
      const part = form.get("body");
      if (!meta || typeof meta !== "object" || Array.isArray(meta) || !(part instanceof Blob)) throw new Error();
      if (part.size > cfg.maxRequestBytes) return tooLarge();
      parsed = { ...meta, body: new Uint8Array(await part.arrayBuffer()) };
    } catch {
      return fail(new ProxyError("Invalid upload: expected a `request` JSON part and a `body` file part", 400, "Bad Request"));
    }
    return send(parsed);
  }

  try {
    const text = await req.text();
    if (Buffer.byteLength(text) > cfg.maxRequestBytes) return tooLarge();
    parsed = JSON.parse(text);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error();
  } catch {
    return fail(new ProxyError("Invalid request body: expected JSON { url, method, headers, body }", 400, "Bad Request"));
  }

  return send(parsed);
}

async function send(parsed: ProxyRequest) {
  try {
    return NextResponse.json(await forward(parsed), { headers: { "Cache-Control": "no-store" } });
  } catch (err) {
    if (err instanceof ProxyError) return fail(err);
    console.error("proxy: unexpected failure", err);
    return fail(new ProxyError("Proxy failed unexpectedly", 500, "Proxy Error"));
  }
}
