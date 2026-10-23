import { describe, it, expect } from "vitest";
import { execFileSync } from "node:child_process";
import { curlConverter, jsonToCurl } from "@/utils/curlConverter";
import type { THittableCurlJson } from "@/types";

const obj = (json: string) => JSON.parse(json);
const req = (over: Partial<THittableCurlJson>): THittableCurlJson => ({
  method: "GET", url: "https://api.example.com", headers: "{}", body: "{}", params: "{}", ...over,
});

describe("curlConverter: pasting a curl command", () => {
  it("returns an empty GET request for an empty string", () => {
    expect(curlConverter("")).toEqual({ method: "GET", url: "", params: "{}", headers: "{}", body: "{}" });
  });

  it("reads method, URL, headers and a JSON body", () => {
    const r = curlConverter(`curl -X POST https://api.example.com/users -H "Content-Type: application/json" -H 'X-Id: 7' -d '{"name":"Ada"}'`);
    expect(r.method).toBe("POST");
    expect(r.url).toBe("https://api.example.com/users");
    expect(obj(r.headers)).toEqual({ "Content-Type": "application/json", "X-Id": "7" });
    expect(obj(r.body)).toEqual({ name: "Ada" });
  });

  it("keeps JSON body types: numbers, booleans, null and nested objects stay as they are", () => {
    const r = curlConverter(`curl -X POST https://x.dev -d '{"n":1,"done":true,"none":null,"o":{"p":[1,"a"]}}'`);
    expect(obj(r.body)).toEqual({ n: 1, done: true, none: null, o: { p: [1, "a"] } });
  });

  it("keeps <<KEY>> templates at the start of the URL, in headers and in the body", () => {
    const r = curlConverter(`curl -X PUT <<BASE_URL>>/users/<<ID>> -H "Authorization: Bearer <<TOKEN>>" -d '{"name":"<<NAME>>"}'`);
    expect(r.method).toBe("PUT");
    expect(r.url).toBe("<<BASE_URL>>/users/<<ID>>");
    expect(obj(r.headers)).toEqual({ Authorization: "Bearer <<TOKEN>>" });
    expect(obj(r.body)).toEqual({ name: "<<NAME>>" });
  });

  it("splits the query string into params, decoded", () => {
    const r = curlConverter(`curl 'https://x.dev/search?q=hello%20world&page=2&flag'`);
    expect(r.url).toBe("https://x.dev/search?q=hello%20world&page=2&flag");
    expect(obj(r.params)).toEqual({ q: "hello world", page: "2", flag: "" });
  });

  it("does not treat a #fragment as part of the last param", () => {
    expect(obj(curlConverter("curl https://x.dev/?a=1#top").params)).toEqual({ a: "1" });
  });

  it("an unquoted URL with & is read whole (no shell splits it here)", () => {
    expect(curlConverter("curl https://x.dev/a?b=1&c=2").url).toBe("https://x.dev/a?b=1&c=2");
  });

  it("defaults to POST when there is a body and no -X, and to GET otherwise", () => {
    expect(curlConverter(`curl https://x.dev -d '{"a":1}'`).method).toBe("POST");
    expect(curlConverter("curl https://x.dev").method).toBe("GET");
  });

  it("accepts long flags, --flag=value and attached short flags", () => {
    const r = curlConverter(`curl --request patch --url https://x.dev --header 'A: 1' --data-raw '{"x":1}'`);
    expect(r).toMatchObject({ method: "PATCH", url: "https://x.dev" });
    expect(obj(r.headers)).toEqual({ A: "1" });
    expect(obj(r.body)).toEqual({ x: 1 });
    expect(curlConverter("curl -XDELETE https://x.dev").method).toBe("DELETE");
    expect(curlConverter("curl --request=OPTIONS https://x.dev").method).toBe("OPTIONS");
  });

  it("supports every data flag", () => {
    for (const flag of ["-d", "--data", "--data-raw", "--data-binary", "--data-ascii", "--data-urlencode"]) {
      expect(obj(curlConverter(`curl https://x.dev ${flag} '{"k":"v"}'`).body)).toEqual({ k: "v" });
    }
  });

  it("--json sets the body and JSON content-type / accept headers", () => {
    const r = curlConverter(`curl https://x.dev --json '{"a":1}'`);
    expect(r.method).toBe("POST");
    expect(obj(r.headers)).toEqual({ "Content-Type": "application/json", Accept: "application/json" });
  });

  it("-u user:pass becomes a Basic Authorization header (UTF-8 safe)", () => {
    expect(obj(curlConverter("curl -u ada:s3cret https://x.dev").headers)).toEqual({
      Authorization: `Basic ${Buffer.from("ada:s3cret").toString("base64")}`,
    });
    expect(obj(curlConverter("curl --user 'jürgen:pä ss' https://x.dev").headers).Authorization).toBe(
      `Basic ${Buffer.from("jürgen:pä ss", "utf8").toString("base64")}`,
    );
  });

  it("-b, -A and -e become Cookie, User-Agent and Referer headers", () => {
    expect(obj(curlConverter("curl https://x.dev -b 'a=1; b=2' -A 'agent/1' -e https://ref.dev").headers)).toEqual({
      Cookie: "a=1; b=2", "User-Agent": "agent/1", Referer: "https://ref.dev",
    });
  });

  it("-I / --head is a HEAD request", () => {
    expect(curlConverter("curl -I https://x.dev").method).toBe("HEAD");
    expect(curlConverter("curl --head https://x.dev").method).toBe("HEAD");
  });

  it("ignores flags without meaning here and skips their arguments", () => {
    const r = curlConverter("curl -sSL --compressed -k -o out.json --max-time 5 -w '%{http_code}' https://x.dev/ok");
    expect(r).toMatchObject({ method: "GET", url: "https://x.dev/ok" });
  });

  it("joins backslash-continued lines, including Windows line endings", () => {
    const r = curlConverter("curl https://x.dev \\\r\n  -H 'A: b' \\\n  -X DELETE");
    expect(r.method).toBe("DELETE");
    expect(obj(r.headers)).toEqual({ A: "b" });
  });

  it("understands shell quoting: '\\'' and '\"'\"' escapes, \\\" inside double quotes", () => {
    expect(obj(curlConverter(`curl https://x.dev -H 'X: it'\\''s' -H "Y: say \\"hi\\""`).headers)).toEqual({
      X: "it's", Y: 'say "hi"',
    });
    expect(obj(curlConverter(`curl https://x.dev -H 'Z: it'"'"'s'`).headers)).toEqual({ Z: "it's" });
  });

  it("reads Chrome's $'…' quoting with escaped newlines", () => {
    const r = curlConverter(`curl 'https://x.dev' --data-raw $'{"text":"a\\\\nb","q":"it\\'s"}'`);
    expect(obj(r.body)).toEqual({ text: "a\nb", q: "it's" });
  });

  it("keeps a header value that contains a colon (only the first colon splits)", () => {
    expect(obj(curlConverter("curl https://x.dev -H 'X-Time: 12:30:00'").headers)).toEqual({ "X-Time": "12:30:00" });
  });

  it("form-encoded data becomes a JSON object body", () => {
    expect(obj(curlConverter("curl -X PUT https://x.dev -d 'a=1&b=two%20words'").body)).toEqual({ a: "1", b: "two words" });
  });

  it("several -d flags are joined with & like curl does", () => {
    expect(obj(curlConverter("curl https://x.dev -d a=1 -d b=2").body)).toEqual({ a: "1", b: "2" });
  });

  it("plain-text data is kept as raw text instead of being mangled into an object", () => {
    expect(curlConverter("curl https://x.dev --data-raw 'hello world'").body).toBe("hello world");
    expect(curlConverter("curl https://x.dev -d '<xml a=\"1\"/>'").body).toBe('<xml a="1"/>');
  });

  it("-F form fields become a JSON object body", () => {
    const r = curlConverter("curl https://x.dev -F name=Ada -F file=@photo.png");
    expect(r.method).toBe("POST");
    expect(obj(r.body)).toEqual({ name: "Ada", file: "@photo.png" });
  });

  it("does not crash on hostile or truncated input", () => {
    for (const s of ["curl '", 'curl "unterminated', "curl -H", "curl -X", "curl \\", "-d", "curl $'\\", "curl -H ':novalue'"]) {
      expect(() => curlConverter(s)).not.toThrow();
    }
  });

  it("works with a bare URL that is not prefixed by curl", () => {
    expect(curlConverter("https://x.dev/a").url).toBe("https://x.dev/a");
  });
});

describe("jsonToCurl: Copy as cURL and saved routes", () => {
  it("single-quotes the URL so & and ? survive a shell", () => {
    expect(jsonToCurl(req({ url: "https://x.dev/?a=1&b=2" }))).toBe("curl -X GET 'https://x.dev/?a=1&b=2'");
  });

  it("writes one -H per header and the body with -d", () => {
    expect(jsonToCurl(req({ method: "POST", headers: '{"A":"1","B":"two"}', body: '{"x":1}' }))).toBe(
      `curl -X POST 'https://api.example.com' -H 'A: 1' -H 'B: two' -d '{"x":1}'`,
    );
  });

  it("escapes single quotes in values the POSIX way", () => {
    expect(jsonToCurl(req({ method: "POST", headers: `{"X":"it's"}`, body: `{"q":"it's"}` }))).toBe(
      `curl -X POST 'https://api.example.com' -H 'X: it'\\''s' -d '{"q":"it'\\''s"}'`,
    );
  });

  it("uses -I for HEAD (curl -X HEAD hangs waiting for a body)", () => {
    expect(jsonToCurl(req({ method: "HEAD" }))).toBe("curl -I 'https://api.example.com'");
  });

  it("never sends a body for GET, HEAD or DELETE", () => {
    for (const method of ["GET", "HEAD", "DELETE", "get"]) {
      expect(jsonToCurl(req({ method, body: '{"a":1}' }))).not.toContain("-d");
    }
  });

  it("skips headers that are not valid JSON instead of throwing", () => {
    expect(jsonToCurl(req({ headers: "{not json" }))).toBe("curl -X GET 'https://api.example.com'");
    expect(jsonToCurl(req({ headers: "null" }))).toBe("curl -X GET 'https://api.example.com'");
  });

  it("uppercases a lowercase method", () => {
    expect(jsonToCurl(req({ method: "post", body: "x" }))).toBe("curl -X POST 'https://api.example.com' -d 'x'");
  });
});

describe("save → reload round trip (jsonToCurl then curlConverter)", () => {
  const cases: [string, THittableCurlJson][] = [
    ["templated URL with headers and JSON body", req({
      method: "POST", url: "<<BASE_URL>>/users?team=<<TEAM>>",
      headers: '{"Authorization":"Bearer <<TOKEN>>","X-Quote":"say \\"hi\\" it\'s"}',
      body: '{"n":1,"ok":true,"tags":["a","b"],"nested":{"k":null}}',
    })],
    ["HEAD request", req({ method: "HEAD", url: "https://x.dev/health" })],
    ["DELETE with headers", req({ method: "DELETE", url: "https://x.dev/u/1", headers: '{"X":"1"}' })],
    ["raw text body", req({ method: "PUT", url: "https://x.dev", body: "line one\nline 'two' $HOME `x`" })],
    ["unicode", req({ method: "POST", url: "https://x.dev/ü", headers: '{"X-Name":"Zoë 🚀"}', body: '{"msg":"héllo 🚀"}' })],
  ];
  it.each(cases)("%s keeps method, URL, headers and body", (_, input) => {
    const back = curlConverter(jsonToCurl(input));
    expect(back.method).toBe(input.method);
    expect(back.url).toBe(input.url);
    expect(obj(back.headers)).toEqual(obj(input.headers));
    if (["GET", "HEAD", "DELETE"].includes(input.method)) expect(back.body).toBe("{}");
    else {
      try {
        expect(obj(back.body)).toEqual(obj(input.body));
      } catch {
        expect(back.body).toBe(input.body);
      }
    }
  });

  it("reads curls saved by older versions (double-quoted headers, unquoted URL)", () => {
    const back = curlConverter(`curl -X POST https://x.dev/a?b=1 -H "X: say \\"hi\\"" -d '{"a":"it'\\''s"}'`);
    expect(back.method).toBe("POST");
    expect(obj(back.headers)).toEqual({ X: 'say "hi"' });
    expect(obj(back.body)).toEqual({ a: "it's" });
  });
});

// The copied command must be something a real shell accepts: run it through
// sh and print the argv curl would receive.
describe("Copy as cURL is valid shell", () => {
  it("a POSIX shell splits it into exactly the intended arguments", () => {
    const cmd = jsonToCurl(req({
      method: "POST", url: "https://x.dev/?a=1&b=$HOME",
      headers: '{"X":"it\'s `whoami` $(id)"}', body: '{"q":"a&b;c|d"}',
    }));
    const argv = execFileSync("sh", ["-c", `printargs() { for a in "$@"; do printf '%s\\0' "$a"; done; }; ${cmd.replace(/^curl/, "printargs")}`])
      .toString().split("\0").slice(0, -1);
    expect(argv).toEqual(["-X", "POST", "https://x.dev/?a=1&b=$HOME", "-H", "X: it's `whoami` $(id)", "-d", '{"q":"a&b;c|d"}']);
  });
});
