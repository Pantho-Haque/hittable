import http from "node:http";
import zlib from "node:zlib";
import type { AddressInfo } from "node:net";
import { afterAll, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { NextRequest } from "next/server";
import {
  ProxyError,
  cleanHeaders,
  createSafeAgent,
  forward,
  isBlockedAddress,
  parseSetCookies,
  rateLimit,
  resetRateLimit,
} from "@/utils/server/proxy";
import { POST } from "@/app/api/proxy/route";

// A stand-in upstream API with one route per behaviour under test.
let server: http.Server;
let base: string;
let port: number;
let lastRequest: { method?: string; headers: http.IncomingHttpHeaders; body: string } = { headers: {}, body: "" };

beforeAll(async () => {
  server = http.createServer((req, res) => {
    let body = "";
    req.on("data", (c) => (body += c));
    req.on("end", () => {
      lastRequest = { method: req.method, headers: req.headers, body };
      const url = new URL(req.url!, "http://x");
      switch (url.pathname) {
        case "/json":
          res.writeHead(200, { "content-type": "application/json; charset=utf-8", "x-custom": "yes" });
          return res.end(JSON.stringify({ hello: "wörld", n: 1 }));
        case "/vnd-json":
          res.writeHead(200, { "content-type": "application/vnd.api+json" });
          return res.end('{"data":[]}');
        case "/bad-json":
          res.writeHead(200, { "content-type": "application/json" });
          return res.end("{not json");
        case "/text":
          res.writeHead(200, { "content-type": "text/plain" });
          return res.end("plain text");
        case "/empty":
          res.writeHead(200, { "content-type": "application/json" });
          return res.end();
        case "/no-content":
          res.writeHead(204);
          return res.end();
        case "/not-found":
          res.writeHead(404, { "content-type": "application/json" });
          return res.end('{"error":"nope"}');
        case "/echo":
          res.writeHead(200, { "content-type": "application/json" });
          return res.end(JSON.stringify({ method: req.method, headers: req.headers, body }));
        case "/cookies":
          res.setHeader("set-cookie", [
            "session=abc123; Path=/; Expires=Wed, 21 Oct 2026 07:28:00 GMT; HttpOnly",
            "theme=dark; Max-Age=3600",
          ]);
          res.writeHead(200, { "content-type": "text/plain" });
          return res.end("ok");
        case "/redirect":
          res.writeHead(302, { location: url.searchParams.get("to") ?? "/json" });
          return res.end();
        case "/post-redirect":
          res.writeHead(303, { location: "/echo" });
          return res.end();
        case "/big":
          res.writeHead(200, { "content-type": "text/plain", "content-length": "5000" });
          return res.end("x".repeat(5000));
        case "/big-chunked":
          res.writeHead(200, { "content-type": "text/plain" }); // no content-length
          for (let i = 0; i < 50; i++) res.write("y".repeat(100));
          return res.end();
        case "/gzip":
          res.writeHead(200, { "content-type": "application/json", "content-encoding": "gzip" });
          return res.end(zlib.gzipSync(JSON.stringify({ compressed: true })));
        case "/slow":
          setTimeout(() => {
            res.writeHead(200);
            res.end("late");
          }, 2000);
          return;
        default:
          res.writeHead(500);
          res.end();
      }
    });
  });
  await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
  port = (server.address() as AddressInfo).port;
  base = `http://127.0.0.1:${port}`;
});

afterAll(() => {
  server.closeAllConnections();
  server.close();
});

const ENV_KEYS = [
  "PROXY_ALLOW_PRIVATE_NETWORKS",
  "PROXY_TIMEOUT_MS",
  "PROXY_MAX_RESPONSE_BYTES",
  "PROXY_MAX_REQUEST_BYTES",
  "PROXY_RATE_LIMIT_PER_MINUTE",
];
beforeEach(() => {
  for (const k of ENV_KEYS) delete process.env[k];
  resetRateLimit();
});

// The local upstream is on loopback, so functional tests opt in to private networks.
const allowLocal = () => (process.env.PROXY_ALLOW_PRIVATE_NETWORKS = "true");

async function expectProxyError(p: Promise<unknown>, httpStatus: number, message?: RegExp) {
  const err = await p.then(
    () => {
      throw new Error("expected a ProxyError");
    },
    (e) => e,
  );
  expect(err).toBeInstanceOf(ProxyError);
  expect((err as ProxyError).httpStatus).toBe(httpStatus);
  if (message) expect((err as ProxyError).message).toMatch(message);
  return err as ProxyError;
}

describe("isBlockedAddress", () => {
  it.each([
    "127.0.0.1", "127.255.255.254", "10.0.0.1", "10.255.255.255", "172.16.0.1", "172.31.255.255",
    "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "224.0.0.1", "255.255.255.255",
    "198.18.0.1", "192.0.2.10",
    "::", "::1", "fe80::1", "fc00::1", "fd12:3456::1", "ff02::1",
    "::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:7f00:1", "64:ff9b::7f00:1", "2002:7f00:1::",
  ])("blocks %s", (ip) => expect(isBlockedAddress(ip)).toBe(true));

  it.each(["8.8.8.8", "1.1.1.1", "93.184.216.34", "172.32.0.1", "172.15.255.255", "2606:4700:4700::1111", "2a00:1450::1"])(
    "allows public %s",
    (ip) => expect(isBlockedAddress(ip)).toBe(false),
  );

  it("treats anything that is not an IP as blocked", () => {
    expect(isBlockedAddress("localhost")).toBe(true);
    expect(isBlockedAddress("")).toBe(true);
  });
});

describe("cleanHeaders", () => {
  it("accepts an object or a JSON string", () => {
    expect(cleanHeaders({ Accept: "application/json" })).toEqual({ Accept: "application/json" });
    expect(cleanHeaders('{"X-A":"1"}')).toEqual({ "X-A": "1" });
    expect(cleanHeaders("")).toEqual({});
    expect(cleanHeaders(undefined)).toEqual({});
    expect(cleanHeaders(null)).toEqual({});
  });

  it("drops hop-by-hop headers whatever their case", () => {
    const out = cleanHeaders({ Host: "evil", "Content-Length": "9", CONNECTION: "close", "Transfer-Encoding": "chunked", "X-Keep": "1" });
    expect(out).toEqual({ "X-Keep": "1" });
  });

  it("stringifies values and skips empty names and null values", () => {
    expect(cleanHeaders({ "X-Num": 5, "X-Bool": true, "X-Null": null, " ": "x" })).toEqual({ "X-Num": "5", "X-Bool": "true" });
  });

  it("rejects malformed headers with a 400", () => {
    for (const bad of ["{oops", "[1,2]", 42]) {
      expect(() => cleanHeaders(bad)).toThrow(ProxyError);
    }
    try {
      cleanHeaders([]);
    } catch (e) {
      expect((e as ProxyError).httpStatus).toBe(400);
    }
  });
});

describe("parseSetCookies", () => {
  it("keeps cookies whose Expires date contains a comma", () => {
    expect(
      parseSetCookies(["a=1; Expires=Wed, 21 Oct 2026 07:28:00 GMT; Path=/", "b=two=parts; HttpOnly"]),
    ).toEqual({ a: "1", b: "two=parts" });
  });

  it("ignores lines without a name", () => {
    expect(parseSetCookies(["=nope", "junk", ""])).toEqual({});
  });
});

describe("rateLimit", () => {
  it("allows up to the limit per key per minute, then reports when to retry", () => {
    const t = 1_000_000;
    expect(rateLimit("a", 2, t).ok).toBe(true);
    expect(rateLimit("a", 2, t + 1).ok).toBe(true);
    const third = rateLimit("a", 2, t + 30_000);
    expect(third).toEqual({ ok: false, retryAfter: 30 });
    expect(rateLimit("b", 2, t + 30_000).ok).toBe(true); // other clients unaffected
    expect(rateLimit("a", 2, t + 60_000).ok).toBe(true); // window rolled over
  });

  it("is off when the limit is 0", () => {
    for (let i = 0; i < 1000; i++) expect(rateLimit("x", 0).ok).toBe(true);
  });
});

describe("forward: requests a user sends", () => {
  beforeEach(allowLocal);

  it("parses a JSON response and returns status, headers and timing-free fields", async () => {
    const r = await forward({ url: `${base}/json`, method: "get" });
    expect(r).toMatchObject({ status: 200, ok: true, statusText: "OK", data: { hello: "wörld", n: 1 }, cookies: {} });
    expect(r.headers["x-custom"]).toBe("yes");
  });

  it("parses +json media types as JSON", async () => {
    expect((await forward({ url: `${base}/vnd-json`, method: "GET" })).data).toEqual({ data: [] });
  });

  it("shows invalid JSON as text instead of failing", async () => {
    expect((await forward({ url: `${base}/bad-json`, method: "GET" })).data).toBe("{not json");
  });

  it("returns text bodies as strings and empty bodies as null", async () => {
    expect((await forward({ url: `${base}/text`, method: "GET" })).data).toBe("plain text");
    expect((await forward({ url: `${base}/empty`, method: "GET" })).data).toBeNull();
    expect((await forward({ url: `${base}/no-content`, method: "DELETE" })).status).toBe(204);
  });

  it("passes non-2xx responses through as answers, not errors", async () => {
    const r = await forward({ url: `${base}/not-found`, method: "GET" });
    expect(r).toMatchObject({ status: 404, ok: false, data: { error: "nope" } });
  });

  it("returns headers as the body of a HEAD request", async () => {
    const r = await forward({ url: `${base}/json`, method: "HEAD" });
    expect(r.status).toBe(200);
    expect((r.data as Record<string, string>)["x-custom"]).toBe("yes");
  });

  it("sends string bodies verbatim and object bodies as JSON", async () => {
    await forward({ url: `${base}/echo`, method: "POST", headers: { "Content-Type": "text/plain" }, body: "raw=1&b=2" });
    expect(lastRequest).toMatchObject({ method: "POST", body: "raw=1&b=2" });

    await forward({ url: `${base}/echo`, method: "PUT", body: { a: [1, 2] } });
    expect(lastRequest).toMatchObject({ method: "PUT", body: '{"a":[1,2]}' });

    await forward({ url: `${base}/echo`, method: "PATCH", body: null });
    expect(lastRequest.body).toBe("");
  });

  it("never sends a body with GET", async () => {
    await forward({ url: `${base}/echo`, method: "GET", body: "ignored" });
    expect(lastRequest).toMatchObject({ method: "GET", body: "" });
  });

  it("forwards the user's headers but not a spoofed Host", async () => {
    await forward({ url: `${base}/echo`, method: "GET", headers: '{"Authorization":"Bearer t0k","Host":"evil.example"}' });
    expect(lastRequest.headers.authorization).toBe("Bearer t0k");
    expect(lastRequest.headers.host).toBe(`127.0.0.1:${port}`);
  });

  it("returns every Set-Cookie, including ones with Expires dates", async () => {
    const r = await forward({ url: `${base}/cookies`, method: "GET" });
    expect(r.cookies).toEqual({ session: "abc123", theme: "dark" });
    expect(r.headers["set-cookie"].split("\n")).toHaveLength(2);
  });

  it("follows redirects", async () => {
    expect((await forward({ url: `${base}/redirect`, method: "GET" })).data).toEqual({ hello: "wörld", n: 1 });
    const r = await forward({ url: `${base}/post-redirect`, method: "POST", body: "x" });
    expect((r.data as { method: string }).method).toBe("GET"); // 303 turns POST into GET
  });

  it("decompresses gzip responses", async () => {
    expect((await forward({ url: `${base}/gzip`, method: "GET" })).data).toEqual({ compressed: true });
  });

  it("accepts any custom method name", async () => {
    await forward({ url: `${base}/echo`, method: "PURGE" });
    expect(lastRequest.method).toBe("PURGE");
  });
});

describe("forward: limits and bad input", () => {
  beforeEach(allowLocal);

  it("refuses responses over the size cap, declared or streamed", async () => {
    process.env.PROXY_MAX_RESPONSE_BYTES = "1000";
    await expectProxyError(forward({ url: `${base}/big`, method: "GET" }), 502, /larger than the proxy limit/);
    await expectProxyError(forward({ url: `${base}/big-chunked`, method: "GET" }), 502, /larger than the proxy limit/);
    // a small response still works under the same cap
    expect((await forward({ url: `${base}/json`, method: "GET" })).status).toBe(200);
  });

  it("times out slow upstreams with a 504", async () => {
    process.env.PROXY_TIMEOUT_MS = "200";
    await expectProxyError(forward({ url: `${base}/slow`, method: "GET" }), 504, /did not answer within 200 ms/);
  });

  it("reports an unreachable host as a network error", async () => {
    const closed = http.createServer();
    await new Promise<void>((r) => closed.listen(0, "127.0.0.1", r));
    const deadPort = (closed.address() as AddressInfo).port;
    closed.close();
    await expectProxyError(forward({ url: `http://127.0.0.1:${deadPort}/`, method: "GET" }), 502, /ECONNREFUSED/);
  });

  it("reports a host that does not resolve", async () => {
    await expectProxyError(forward({ url: "http://does-not-exist.invalid/", method: "GET" }), 502, /ENOTFOUND|EAI_AGAIN/);
  });

  it.each([
    [{ url: "", method: "GET" }, /Missing required fields/],
    [{ url: "BASE/json", method: "" }, /Missing required fields/],
    [{ url: 42, method: "GET" }, /Missing required fields/],
    [{ url: "not a url", method: "GET" }, /Invalid URL/],
    [{ url: "ftp://example.com/file", method: "GET" }, /Only http and https/],
    [{ url: "file:///etc/passwd", method: "GET" }, /Only http and https/],
    [{ url: "javascript:alert(1)", method: "GET" }, /Only http and https/],
    [{ url: "BASE/json", method: "CONNECT" }, /not allowed/],
    [{ url: "BASE/json", method: "trace" }, /not allowed/],
    [{ url: "BASE/json", method: "GE T" }, /not allowed/],
    [{ url: "BASE/json", method: "GET", headers: "{broken" }, /Headers must be a JSON object/],
  ])("rejects %o with a 400", async (req, msg) => {
    const url = typeof req.url === "string" ? req.url.replace("BASE", base) : req.url;
    await expectProxyError(forward({ ...req, url }), 400, msg);
  });

  it("rejects header values that would split the request", async () => {
    await expectProxyError(forward({ url: `${base}/echo`, method: "GET", headers: { "X-A": "a\r\nInjected: 1" } }), 400);
  });
});

describe("forward: private network protection (the public default)", () => {
  it.each([
    "http://127.0.0.1/",
    "http://localhost/",
    "http://LOCALHOST./",
    "http://[::1]/",
    "http://[::ffff:127.0.0.1]/",
    "http://169.254.169.254/latest/meta-data/",
    "http://10.0.0.5:8080/",
    "http://192.168.0.1/",
    "http://0.0.0.0/",
    "http://0x7f000001/", // WHATWG URL normalises these to 127.0.0.1
    "http://2130706433/",
    "http://017700000001/",
  ])("refuses %s with a 403 before connecting", async (url) => {
    await expectProxyError(forward({ url, method: "GET" }), 403, /private or reserved address/);
  });

  it("refuses a redirect that points into the private network", async () => {
    // Treat the test server's own address as "public" to stand in for the internet.
    const agent = createSafeAgent((ip) => ip !== "127.0.0.1" && isBlockedAddress(ip));
    expect((await forward({ url: `${base}/json`, method: "GET" }, { dispatcher: agent })).status).toBe(200);

    for (const to of [`http://127.0.0.2:${port}/json`, `http://[::1]:${port}/json`, `http://169.254.169.254/`]) {
      await expectProxyError(
        forward({ url: `${base}/redirect?to=${encodeURIComponent(to)}`, method: "GET" }, { dispatcher: agent }),
        403,
      );
    }
  });

  it("refuses a hostname when any of its addresses is private", async () => {
    // localhost resolves to 127.0.0.1 and ::1; even with 127.0.0.1 allowed, ::1 blocks it.
    const agent = createSafeAgent((ip) => ip !== "127.0.0.1" && isBlockedAddress(ip));
    await expectProxyError(forward({ url: `http://localhost:${port}/json`, method: "GET" }, { dispatcher: agent }), 403);
  });
});

describe("POST /api/proxy", () => {
  const call = (body: string, headers: Record<string, string> = {}) =>
    POST(new NextRequest("http://app.test/api/proxy", { method: "POST", body, headers: { "content-type": "application/json", ...headers } }));

  it("returns the upstream response in the standard shape", async () => {
    allowLocal();
    const res = await call(JSON.stringify({ url: `${base}/json`, method: "GET", headers: "{}" }));
    expect(res.status).toBe(200);
    expect(res.headers.get("cache-control")).toBe("no-store");
    expect(await res.json()).toMatchObject({ status: 200, ok: true, data: { hello: "wörld", n: 1 } });
  });

  it.each(["", "not json", "[1,2]", "null", '"str"'])("rejects body %j with a 400 in the standard shape", async (body) => {
    const res = await call(body);
    expect(res.status).toBe(400);
    expect(await res.json()).toEqual({
      data: null,
      status: 0,
      statusText: "Bad Request",
      ok: false,
      headers: {},
      cookies: {},
      error: expect.stringMatching(/Invalid request body/),
    });
  });

  it("blocks private targets by default", async () => {
    const res = await call(JSON.stringify({ url: `${base}/json`, method: "GET" }));
    expect(res.status).toBe(403);
    expect((await res.json()).error).toMatch(/private or reserved/);
  });

  it("refuses oversized requests with a 413", async () => {
    process.env.PROXY_MAX_REQUEST_BYTES = "100";
    const big = JSON.stringify({ url: "https://example.com", method: "POST", body: "z".repeat(500) });
    const res = await call(big);
    expect(res.status).toBe(413);
    const declared = await call("{}", { "content-length": "999999" });
    expect(declared.status).toBe(413);
  });

  it("rate limits each client and says when to retry", async () => {
    process.env.PROXY_RATE_LIMIT_PER_MINUTE = "2";
    const body = JSON.stringify({ url: "ftp://x", method: "GET" });
    expect((await call(body, { "x-forwarded-for": "1.2.3.4, 10.0.0.1" })).status).toBe(400);
    expect((await call(body, { "x-forwarded-for": "1.2.3.4" })).status).toBe(400);
    const limited = await call(body, { "x-forwarded-for": "1.2.3.4" });
    expect(limited.status).toBe(429);
    expect(Number(limited.headers.get("retry-after"))).toBeGreaterThan(0);
    expect((await limited.json()).error).toMatch(/Too many requests/);
    expect((await call(body, { "x-forwarded-for": "5.6.7.8" })).status).toBe(400); // someone else is fine
  });

  it("maps upstream failures to gateway status codes", async () => {
    allowLocal();
    process.env.PROXY_TIMEOUT_MS = "150";
    const res = await call(JSON.stringify({ url: `${base}/slow`, method: "GET" }));
    expect(res.status).toBe(504);
    expect(await res.json()).toMatchObject({ status: 0, ok: false, statusText: "Timeout" });
  });
});
