import { describe, it, expect } from "vitest";
import pako from "pako";
import { compressString } from "@/utils/compressString";
import { decodeShare, encodeShare, redactHeader, shareUrl, SHARE_MAX_BYTES } from "@/utils/shareLink";
import type { THittableCurlJson } from "@/types";

const req = (over: Partial<THittableCurlJson> = {}): THittableCurlJson => ({
  method: "POST",
  url: "https://<<HOST>>/users?q=héllo",
  headers: JSON.stringify({ "Content-Type": "application/json", "X-Trace": "<<TRACE>>" }),
  params: JSON.stringify({ q: "héllo" }),
  body: '{"name":"Zoë 🚀","token":"<<TOKEN>>"}',
  ...over,
});

const encoded = (r: THittableCurlJson, name?: string) => {
  const res = encodeShare(r, name);
  if (!res.ok) throw new Error(res.error);
  return res;
};

const headersOf = (r: THittableCurlJson) => {
  const d = decodeShare(encoded(r).fragment);
  if (!d.ok) throw new Error(d.error);
  return JSON.parse(d.request.headers);
};

/** A fragment built from any payload, as a forger would. */
const forge = (payload: unknown) => `share=${compressString(typeof payload === "string" ? payload : JSON.stringify(payload))}`;
const valid = { v: 1, name: "", method: "GET", url: "https://a.io", headers: {}, params: {}, body: "" };

describe("encodeShare / decodeShare", () => {
  it("round-trips method, url, headers, params and body, unicode and <<KEY>> included", () => {
    const r = req();
    const { fragment, redacted } = encoded(r, "Create user");
    expect(fragment).toMatch(/^share=[A-Za-z0-9_-]+$/);
    expect(redacted).toEqual([]);
    const d = decodeShare(fragment);
    expect(d.ok).toBe(true);
    if (!d.ok) return;
    expect(d.name).toBe("Create user");
    expect(d.request.method).toBe("POST");
    expect(d.request.url).toBe(r.url);
    expect(JSON.parse(d.request.headers)).toEqual(JSON.parse(r.headers));
    expect(JSON.parse(d.request.params)).toEqual({ q: "héllo" });
    expect(d.request.body).toBe(r.body);
  });

  it("accepts a leading # and empty headers/params", () => {
    const { fragment } = encoded(req({ headers: "", params: "" }));
    const d = decodeShare(`#${fragment}`);
    expect(d.ok && JSON.parse(d.request.headers)).toEqual({});
  });

  it("refuses headers that are not a JSON object", () => {
    const res = encodeShare(req({ headers: "{oops" }));
    expect(res.ok).toBe(false);
    expect(encodeShare(req({ headers: '{"a":{"b":1}}' })).ok).toBe(false);
  });

  it("builds the /hittable URL", () => {
    expect(shareUrl("https://h.io", "share=abc")).toBe("https://h.io/hittable#share=abc");
  });
});

describe("redaction", () => {
  it.each([
    ["Authorization", "Bearer abc", "Bearer <<TOKEN>>"],
    ["authorization", "Basic dXNlcjpwYXNz", "Basic <<BASIC_AUTH>>"],
    ["Authorization", "Digest x=1", "Digest <<AUTHORIZATION>>"],
    ["Proxy-Authorization", "Bearer abc", "Bearer <<TOKEN>>"],
    ["Cookie", "sid=123; theme=dark", "<<COOKIE>>"],
    ["X-Api-Key", "k-123", "<<X_API_KEY>>"],
    ["api-key", "k", "<<API_KEY>>"],
    ["X-Auth-Token", "t", "<<X_AUTH_TOKEN>>"],
    ["X-Access-Token", "t", "<<X_ACCESS_TOKEN>>"],
    ["X-Client-Secret", "s", "<<X_CLIENT_SECRET>>"],
    ["Refresh-Token", "r", "<<REFRESH_TOKEN>>"],
  ])("%s: %s → %s", (name, value, expected) => {
    expect(redactHeader(name, value)).toBe(expected);
    expect(headersOf(req({ headers: JSON.stringify({ [name]: value }) }))[name]).toBe(expected);
  });

  it.each([
    ["Authorization", "Bearer <<TOKEN>>"],
    ["Authorization", "<<AUTH>>"],
    ["Cookie", "<<SESSION>>"],
    ["X-Api-Key", " <<A>><<B>> "],
    ["Content-Type", "application/json"],
    ["X-Request-Id", "abc"],
  ])("leaves %s: %s untouched", (name, value) => {
    expect(redactHeader(name, value)).toBeNull();
    expect(headersOf(req({ headers: JSON.stringify({ [name]: value }) }))[name]).toBe(value);
  });

  it("says which headers were replaced", () => {
    const { redacted } = encoded(req({ headers: JSON.stringify({ Authorization: "Bearer abc", "X-Api-Key": "k" }) }));
    expect(redacted).toEqual(["Authorization was replaced with Bearer <<TOKEN>>", "X-Api-Key was replaced with <<X_API_KEY>>"]);
  });

  it("never carries the literal credential", () => {
    const { fragment } = encoded(req({ headers: JSON.stringify({ Authorization: "Bearer s3cr3t-value" }) }));
    expect(JSON.stringify(decodeShare(fragment))).not.toContain("s3cr3t-value");
  });

  it("strips user:password from the URL", () => {
    const res = encoded(req({ url: "https://ann:pa55@api.io/x?y=1" }));
    const d = decodeShare(res.fragment);
    expect(d.ok && d.request.url).toBe("https://api.io/x?y=1");
    expect(res.redacted.join(" ")).toMatch(/password in the URL were removed/);
    expect(decodeShare(encoded(req({ url: "<<BASE>>/a@b" })).fragment)).toMatchObject({ request: { url: "<<BASE>>/a@b" } });
  });
});

describe("limits and hostile input", () => {
  it("refuses a link over 32 KB with a message", () => {
    // Random-ish text barely compresses.
    let body = "";
    for (let i = 0; body.length < 60_000; i++) body += Math.sin(i).toString(36);
    const res = encodeShare(req({ body }));
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toMatch(/too large.*32 KB/);
  });

  it.each([
    ["empty", ""],
    ["not base64", "share=@@@!!"],
    ["not deflate", `share=${btoa("hello world").replace(/=+$/, "")}`],
    ["truncated", forge(valid).slice(0, 30)],
    ["over the size cap", `share=${"A".repeat(SHARE_MAX_BYTES + 1)}`],
    ["not JSON", forge("{nope")],
    ["JSON null", forge("null")],
    ["wrong version", forge({ ...valid, v: 2 })],
    ["unknown method", forge({ ...valid, method: "TRACE" })],
    ["lowercase method", forge({ ...valid, method: "get" })],
    ["url not a string", forge({ ...valid, url: 42 })],
    ["huge url", forge({ ...valid, url: "x".repeat(10_000) })],
    ["name not a string", forge({ ...valid, name: ["a"] })],
    ["body object", forge({ ...valid, body: { a: 1 } })],
    ["headers array", forge({ ...valid, headers: ["a"] })],
    ["headers nested", forge({ ...valid, headers: { a: { b: "c" } } })],
    ["header value number", forge({ ...valid, headers: { a: 1 } })],
    ["too many headers", forge({ ...valid, headers: Object.fromEntries(Array.from({ length: 101 }, (_, i) => [`h${i}`, "v"])) })],
    ["params missing", forge({ ...valid, params: undefined })],
  ])("%s → ok:false, no throw", (_label, fragment) => {
    const d = decodeShare(fragment);
    expect(d.ok).toBe(false);
    if (!d.ok) expect(d.error).toMatch(/damaged or too large/);
  });

  it("stops a deflate bomb instead of inflating it", () => {
    const bomb = pako.deflate(new Uint8Array(20_000_000).fill(32));
    let bin = "";
    bomb.forEach((b) => (bin += String.fromCharCode(b)));
    const fragment = `share=${btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "")}`;
    expect(fragment.length).toBeLessThan(SHARE_MAX_BYTES);
    expect(decodeShare(fragment).ok).toBe(false);
  });

  it("keeps a __proto__ header as plain data", () => {
    const d = decodeShare(forge('{"v":1,"name":"","method":"GET","url":"u","headers":{"__proto__":"x"},"params":{},"body":""}'));
    expect(d.ok && Object.keys(JSON.parse(d.request.headers))).toEqual(["__proto__"]);
    expect(({} as Record<string, unknown>).x).toBeUndefined();
  });
});

// Credentials in the query string are as sensitive as an Authorization header.
describe("query-string credentials", () => {
  const req = (url: string, params: Record<string, string>) => ({ method: "GET", url, headers: "{}", params: JSON.stringify(params), body: "" });

  it("are replaced with variables in the URL and in Params", () => {
    const r = encodeShare(req("https://x.io/v1?q=cats&api_key=abc123&access_token=t0k", { q: "cats", api_key: "abc123", access_token: "t0k" }));
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    const d = decodeShare(r.fragment);
    expect(d.ok).toBe(true);
    if (!d.ok) return;
    expect(d.request.url).toBe("https://x.io/v1?q=cats&api_key=<<API_KEY>>&access_token=<<ACCESS_TOKEN>>");
    expect(JSON.parse(d.request.params)).toEqual({ q: "cats", api_key: "<<API_KEY>>", access_token: "<<ACCESS_TOKEN>>" });
    expect(r.redacted.join(" ")).toMatch(/api_key/);
  });

  it("leave ordinary parameters and existing variables alone", () => {
    const r = encodeShare(req("https://x.io/?keyword=a&page=2&token=<<TOKEN>>", { keyword: "a", page: "2", token: "<<TOKEN>>" }));
    expect(r.ok && r.redacted).toEqual([]);
  });
});
