import { describe, it, expect } from "vitest";
import { generateCode, CODE_LANGUAGES, CodeLanguage } from "@/utils/codegen";
import { curlConverter } from "@/utils/curlConverter";
import { THittableCurlJson } from "@/types";

const req = (r: Partial<THittableCurlJson>): THittableCurlJson => ({
  method: "GET",
  url: "",
  headers: "{}",
  body: "",
  params: "{}",
  ...r,
});

const getReq = req({
  url: "https://api.example.com/users?page=2&q=a%20b",
  headers: JSON.stringify({ Authorization: "Bearer t", Accept: "application/json" }),
  body: '{"ignored":true}',
});
const jsonReq = req({
  method: "POST",
  url: "https://api.example.com/users",
  headers: JSON.stringify({ "Content-Type": "application/json" }),
  body: JSON.stringify({ name: "Ann", tags: ["a"], admin: false, boss: null, n: 3 }),
});
const formReq = req({
  method: "POST",
  url: "https://api.example.com/login",
  headers: JSON.stringify({ "Content-Type": "application/x-www-form-urlencoded" }),
  body: "a=1&b=x+y",
});
const textReq = req({
  method: "PUT",
  url: "https://api.example.com/note",
  headers: JSON.stringify({ "Content-Type": "text/plain" }),
  body: "hello\nworld",
});
const nastyReq = req({
  method: "POST",
  url: "https://api.example.com/it's",
  headers: JSON.stringify({ "X-Note": "it's $HOME `id`" }),
  body: "it's $HOME `whoami` \\n",
});

// Runs a generated fetch snippet against a recording fetch.
async function runFetch(code: string) {
  const calls: [string, RequestInit][] = [];
  const fakeFetch = async (url: string, init: RequestInit) => {
    calls.push([url, init]);
    return { status: 200, text: async () => "" };
  };
  const fn = new Function("fetch", "console", `return (async () => {\n${code}\n})();`);
  await fn(fakeFetch, { log: () => {} });
  return calls[0];
}

describe("CODE_LANGUAGES", () => {
  it("lists the four generators with labels", () => {
    expect(CODE_LANGUAGES.map((l) => l.id)).toEqual(["curl", "fetch", "python", "httpie"]);
    for (const l of CODE_LANGUAGES) expect(l.label).toBeTruthy();
  });
});

describe("curl", () => {
  const roundTrip = (r: THittableCurlJson) => curlConverter(generateCode("curl", r));

  it("GET with query and headers round-trips, without a body", () => {
    const code = generateCode("curl", getReq);
    expect(code).not.toContain("--data");
    const back = roundTrip(getReq);
    expect(back.method).toBe("GET");
    expect(back.url).toBe(getReq.url);
    expect(JSON.parse(back.headers)).toEqual(JSON.parse(getReq.headers));
    expect(JSON.parse(back.params)).toEqual({ page: "2", q: "a b" });
  });

  it("POST JSON round-trips", () => {
    const back = roundTrip(jsonReq);
    expect(back.method).toBe("POST");
    expect(back.url).toBe(jsonReq.url);
    expect(JSON.parse(back.headers)).toEqual(JSON.parse(jsonReq.headers));
    expect(JSON.parse(back.body)).toEqual(JSON.parse(jsonReq.body));
  });

  // With its urlencoded Content-Type, form data comes back exactly as sent
  // (a Form body), not converted to a JSON object.
  it("POST form round-trips exactly", () => {
    const code = generateCode("curl", formReq);
    expect(code).toContain(`--data-raw 'a=1&b=x+y'`);
    const back = roundTrip(formReq);
    expect(back.method).toBe("POST");
    expect(JSON.parse(back.headers)).toEqual(JSON.parse(formReq.headers));
    expect(back.body).toBe("a=1&b=x+y");
  });

  it("PUT text round-trips exactly", () => {
    const back = roundTrip(textReq);
    expect(back.method).toBe("PUT");
    expect(back.body).toBe(textReq.body);
  });

  it("quotes ', $ and backticks so the shell passes them literally", () => {
    const code = generateCode("curl", nastyReq);
    expect(code).toContain(`'it'\\''s $HOME \`whoami\` \\n'`);
    const back = roundTrip(nastyReq);
    expect(back.url).toBe(nastyReq.url);
    expect(JSON.parse(back.headers)).toEqual(JSON.parse(nastyReq.headers));
    expect(back.body).toBe(nastyReq.body);
  });

  it("HEAD uses -I and no body", () => {
    const code = generateCode("curl", { ...jsonReq, method: "HEAD" });
    expect(code.startsWith("curl -I ")).toBe(true);
    expect(code).not.toContain("--data");
    expect(roundTrip({ ...jsonReq, method: "HEAD" }).method).toBe("HEAD");
  });
});

describe("fetch", () => {
  it("GET with query and headers, no body", async () => {
    const [url, init] = await runFetch(generateCode("fetch", getReq));
    expect(url).toBe(getReq.url);
    expect(init.method).toBe("GET");
    expect(init.headers).toEqual(JSON.parse(getReq.headers));
    expect(init.body).toBeUndefined();
  });

  it("POST JSON uses a pretty JSON.stringify literal", async () => {
    const code = generateCode("fetch", jsonReq);
    expect(code).toContain("body: JSON.stringify({\n");
    const [url, init] = await runFetch(code);
    expect(url).toBe(jsonReq.url);
    expect(init.method).toBe("POST");
    expect(init.headers).toEqual(JSON.parse(jsonReq.headers));
    expect(JSON.parse(init.body as string)).toEqual(JSON.parse(jsonReq.body));
  });

  it("POST form and text bodies are sent verbatim", async () => {
    expect((await runFetch(generateCode("fetch", formReq)))[1].body).toBe("a=1&b=x+y");
    const [, init] = await runFetch(generateCode("fetch", textReq));
    expect(init.method).toBe("PUT");
    expect(init.body).toBe("hello\nworld");
  });

  it("escapes quotes, $ and backticks as plain string content", async () => {
    const [url, init] = await runFetch(generateCode("fetch", nastyReq));
    expect(url).toBe(nastyReq.url);
    expect(init.headers).toEqual(JSON.parse(nastyReq.headers));
    expect(init.body).toBe(nastyReq.body);
  });
});

describe("python", () => {
  it("GET with query and headers, no body", () => {
    const code = generateCode("python", getReq);
    expect(code).toContain("import requests");
    expect(code).toContain(`url = "https://api.example.com/users?page=2&q=a%20b"`);
    expect(code).toContain(`"Authorization": "Bearer t",`);
    expect(code).toContain(`requests.request("GET", url, headers=headers)`);
    expect(code).not.toContain("payload");
  });

  it("POST JSON uses json= with Python literals", () => {
    const code = generateCode("python", jsonReq);
    expect(code).toContain(`requests.request("POST", url, headers=headers, json=payload)`);
    expect(code).toContain(`"admin": False,`);
    expect(code).toContain(`"boss": None,`);
    expect(code).toContain(`"tags": [\n        "a",\n    ],`);
  });

  it("POST form and text use data= with the raw string", () => {
    expect(generateCode("python", formReq)).toContain(`payload = "a=1&b=x+y"`);
    expect(generateCode("python", formReq)).toContain("data=payload");
    const text = generateCode("python", textReq);
    expect(text).toContain(`payload = "hello\\nworld"`);
    expect(text).toContain(`requests.request("PUT", url, headers=headers, data=payload)`);
  });

  it("escapes quotes in string literals", () => {
    const code = generateCode("python", { ...nastyReq, headers: JSON.stringify({ q: `say "hi"` }) });
    expect(code).toContain(`"q": "say \\"hi\\""`);
  });
});

describe("httpie", () => {
  it("GET with query and headers, no body", () => {
    const code = generateCode("httpie", getReq);
    expect(code).toBe(
      `http GET 'https://api.example.com/users?page=2&q=a%20b' \\\n  'Authorization:Bearer t' \\\n  'Accept:application/json'`,
    );
  });

  it("POST JSON, form and text send the raw body", () => {
    expect(generateCode("httpie", jsonReq)).toContain(`--raw '${jsonReq.body}'`);
    expect(generateCode("httpie", jsonReq)).toContain(`'Content-Type:application/json'`);
    expect(generateCode("httpie", formReq)).toContain(`--raw 'a=1&b=x+y'`);
    expect(generateCode("httpie", textReq)).toMatch(/^http PUT /);
    expect(generateCode("httpie", textReq)).toContain(`--raw 'hello\nworld'`);
  });

  it("quotes ', $ and backticks", () => {
    const code = generateCode("httpie", nastyReq);
    expect(code).toContain(`'https://api.example.com/it'\\''s'`);
    expect(code).toContain(`'X-Note:it'\\''s $HOME \`id\`'`);
    expect(code).toContain(`--raw 'it'\\''s $HOME \`whoami\` \\n'`);
  });

  it("sends an empty header with `;`", () => {
    expect(generateCode("httpie", req({ headers: '{"X-Empty":""}' }))).toContain(`'X-Empty;'`);
  });
});

describe("all languages", () => {
  const langs: CodeLanguage[] = ["curl", "fetch", "python", "httpie"];

  it("resolve variables with secrets winning over env", () => {
    const templ = req({
      method: "POST",
      url: "https://<<HOST>>/v1",
      headers: JSON.stringify({ Authorization: "Bearer <<TOKEN>>", "Content-Type": "text/plain" }),
      body: "user=<<USER>>&missing=<<NOPE>>",
    });
    const vars = { ...{ HOST: "env.io", TOKEN: "env-token", USER: "u1" }, ...{ TOKEN: "secret-token" } };
    for (const lang of langs) {
      const code = generateCode(lang, templ, vars);
      expect(code, lang).toContain("env.io/v1");
      expect(code, lang).toContain("secret-token");
      expect(code, lang).not.toContain("env-token");
      expect(code, lang).toContain("user=u1&missing=<<NOPE>>");
    }
  });

  it("leave <<VARS>> in place without vars", () => {
    for (const lang of langs) expect(generateCode(lang, req({ url: "https://<<HOST>>/" }))).toContain("<<HOST>>");
  });

  it("never include a body for GET or HEAD", () => {
    for (const lang of langs) {
      for (const method of ["GET", "HEAD"]) {
        const code = generateCode(lang, { ...jsonReq, method });
        expect(code, `${lang} ${method}`).not.toContain("Ann");
      }
    }
  });

  it("tolerate empty or invalid headers JSON", () => {
    for (const lang of langs) {
      expect(() => generateCode(lang, req({ url: "https://x.io", headers: "" }))).not.toThrow();
      expect(generateCode(lang, req({ url: "https://x.io", headers: "{bad" }))).not.toContain("bad");
    }
  });
});
