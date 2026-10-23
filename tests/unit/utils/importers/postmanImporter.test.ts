import { describe, it, expect } from "vitest";
import { parsePostmanCollection, isPostmanCollection } from "@/utils/importers/postmanImporter";
import { curlConverter } from "@/utils/curlConverter";
import type { THittableCollection, THittableItem, THittableRoute } from "@/types";

const SCHEMA = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json";
const col = (extra: Record<string, unknown>) => ({ info: { name: "Shop", schema: SCHEMA }, ...extra });
const one = (json: unknown): THittableCollection => {
  const out = parsePostmanCollection(json);
  expect(out).toHaveLength(1);
  return out[0];
};
const route = (items: THittableItem[], ...path: string[]): THittableRoute => {
  let level = items;
  for (const name of path.slice(0, -1)) {
    const f = level.find((i) => i.type === "folder" && i.name === name);
    if (!f || f.type !== "folder") throw new Error(`no folder ${name}`);
    level = f.items;
  }
  const r = level.find((i) => i.type === "route" && i.name === path[path.length - 1]);
  if (!r || r.type !== "route") throw new Error(`no route ${path.join("/")}`);
  return r;
};
const parsed = (r: THittableRoute) => {
  const c = curlConverter(r.curl);
  return { ...c, headers: JSON.parse(c.headers), params: JSON.parse(c.params) };
};
const request = (req: Record<string, unknown>, name = "R") => col({ item: [{ name, request: req }] });

describe("isPostmanCollection", () => {
  it("recognises the v2.1 schema URL", () => {
    expect(isPostmanCollection({ info: { schema: SCHEMA }, item: [] })).toBe(true);
    expect(isPostmanCollection({ info: { schema: "https://schema.getpostman.com/json/collection/v2.0.0/" } })).toBe(true);
  });
  it("recognises a schema-less collection with info.name and an item array", () => {
    expect(isPostmanCollection({ info: { name: "x" }, item: [] })).toBe(true);
  });
  it("rejects Insomnia, Hittable and junk", () => {
    for (const v of [null, undefined, 1, "x", [], {}, { info: "x" }, { info: { name: "x" } }, { _type: "export", resources: [] }, { collectionName: "x", items: [] }]) {
      expect(isPostmanCollection(v)).toBe(false);
    }
  });
});

describe("parsePostmanCollection", () => {
  it("maps one Postman collection to one Hittable collection, keeping folder nesting", () => {
    const c = one(col({
      item: [
        { name: "Users", item: [
          { name: "List", request: { method: "GET", url: "https://api.shop.dev/users" } },
          { name: "Admin", item: [{ name: "Ban", request: { method: "POST", url: "https://api.shop.dev/ban" } }] },
        ] },
        { name: "Orders", item: [{ name: "All", request: { method: "GET", url: "https://api.shop.dev/orders" } }] },
      ],
    }));
    expect(c.collectionName).toBe("Shop");
    expect(c.items.map((i) => [i.type, i.name])).toEqual([["folder", "Users"], ["folder", "Orders"]]);
    expect(route(c.items, "Users", "Admin", "Ban").type).toBe("route");
    expect(parsed(route(c.items, "Users", "Admin", "Ban")).method).toBe("POST");
    expect(c.secrets).toEqual({});
  });

  it("keeps top-level requests and folders side by side", () => {
    const c = one(col({ item: [
      { name: "Health", request: { method: "GET", url: "https://x.dev/health" } },
      { name: "F", item: [] },
    ] }));
    expect(c.items.map((i) => i.name)).toEqual(["Health", "F"]);
  });

  it("translates {{var}} to <<var>> in URL, headers and body, and collects variables", () => {
    const c = one(col({
      variable: [{ key: "baseUrl", value: "https://api.dev" }, { key: "off", value: "x", enabled: false }, { key: "empty", value: "" }],
      item: [{ name: "Create", request: {
        method: "POST",
        url: { raw: "{{baseUrl}}/users?team={{ team }}" },
        header: [{ key: "Authorization", value: "Bearer {{token}}" }],
        body: { mode: "raw", raw: '{"name":"{{name}}","n":1}' },
      } }],
    }));
    const p = parsed(route(c.items, "Create"));
    expect(p.url).toBe("<<baseUrl>>/users?team=<<team>>");
    expect(p.headers).toEqual({ Authorization: "Bearer <<token>>" });
    expect(JSON.parse(p.body)).toEqual({ name: "<<name>>", n: 1 });
    // collection values, an explicitly empty variable, and empty entries for referenced-but-undefined ones
    expect(c.env).toEqual({ baseUrl: "https://api.dev", empty: "", team: "", token: "", name: "" });
  });

  it("collects folder-level variables too", () => {
    const c = one(col({ item: [{ name: "F", variable: [{ key: "fv", value: "1" }], item: [] }] }));
    expect(c.env).toEqual({ fv: "1" });
  });

  it("builds the URL from host/path/query parts when there is no raw, skipping disabled query rows", () => {
    const c = one(request({ method: "GET", url: {
      protocol: "https", host: ["api", "example", "com"], port: "8443", path: ["v1", { value: "users" }],
      query: [{ key: "q", value: "a b&c" }, { key: "off", value: "1", disabled: true }, { key: "t", value: "{{tok}}" }],
    } }));
    const p = parsed(route(c.items, "R"));
    expect(p.url).toBe("https://api.example.com:8443/v1/users?q=a%20b%26c&t=<<tok>>");
    expect(p.params).toEqual({ q: "a b&c", t: "<<tok>>" });
  });

  it("treats the query array as authoritative over the raw string", () => {
    const c = one(request({ method: "GET", url: { raw: "https://x.dev/a?q=a b&c", query: [{ key: "q", value: "a b&c" }] } }));
    expect(parsed(route(c.items, "R")).params).toEqual({ q: "a b&c" });
  });

  it("skips disabled headers and keeps the rest", () => {
    const c = one(request({ method: "GET", url: "https://x.dev", header: [
      { key: "A", value: "1" }, { key: "B", value: "2", disabled: true }, { value: "no key" },
    ] }));
    expect(parsed(route(c.items, "R")).headers).toEqual({ A: "1" });
  });

  it("urlencoded bodies become a JSON object and set the form content type", () => {
    const c = one(request({ method: "POST", url: "https://x.dev", body: { mode: "urlencoded", urlencoded: [
      { key: "a", value: "1" }, { key: "b", value: "{{v}}" }, { key: "c", value: "x", disabled: true },
    ] } }));
    const p = parsed(route(c.items, "R"));
    expect(JSON.parse(p.body)).toEqual({ a: "1", b: "<<v>>" });
    expect(p.headers["Content-Type"]).toBe("application/x-www-form-urlencoded");
  });

  it("formdata file rows become @filename references", () => {
    const c = one(request({ method: "POST", url: "https://x.dev", body: { mode: "formdata", formdata: [
      { key: "name", value: "Ada", type: "text" },
      { key: "photo", type: "file", src: "/tmp/me.png" },
      { key: "docs", type: "file", src: ["a.pdf", "b.pdf"] },
      { key: "blank", type: "file", src: null },
    ] } }));
    expect(JSON.parse(parsed(route(c.items, "R")).body)).toEqual({ name: "Ada", photo: "@/tmp/me.png", docs: "@a.pdf", blank: "@filename" });
  });

  it("drops graphql and file bodies, and non-JSON raw bodies stay as text", () => {
    const gql = one(request({ method: "POST", url: "https://x.dev", body: { mode: "graphql", graphql: { query: "{a}" } } }));
    expect(parsed(route(gql.items, "R")).body).toBe("{}");
    const xml = one(request({ method: "POST", url: "https://x.dev", body: { mode: "raw", raw: "<a>{{v}}</a>" } }));
    expect(parsed(route(xml.items, "R")).body).toBe("<a><<v>></a>");
  });

  it("does not attach a body to GET, HEAD or DELETE", () => {
    for (const method of ["GET", "HEAD", "DELETE"]) {
      const c = one(request({ method, url: "https://x.dev", body: { mode: "raw", raw: '{"a":1}' } }));
      expect(route(c.items, "R").curl).not.toContain("-d");
    }
  });

  it("keeps header values with quotes and single quotes intact", () => {
    const c = one(request({ method: "POST", url: "https://x.dev", header: [{ key: "X", value: `say "hi" it's` }], body: { mode: "raw", raw: `{"q":"it's"}` } }));
    const p = parsed(route(c.items, "R"));
    expect(p.headers).toEqual({ X: `say "hi" it's` });
    expect(JSON.parse(p.body)).toEqual({ q: "it's" });
  });

  it("a string request is a GET to that URL", () => {
    const c = one(col({ item: [{ name: "S", request: "https://x.dev/{{p}}" }] }));
    expect(parsed(route(c.items, "S"))).toMatchObject({ method: "GET", url: "https://x.dev/<<p>>" });
    expect(c.env).toEqual({ p: "" });
  });

  it("defaults missing names and methods", () => {
    const c = one({ info: { schema: SCHEMA }, item: [{ request: { url: "https://x.dev" } }, { item: [] }] });
    expect(c.collectionName).toBe("Imported Collection");
    expect(c.items.map((i) => i.name)).toEqual(["Unnamed", "Unnamed Folder"]);
    expect(parsed(route(c.items, "Unnamed")).method).toBe("GET");
  });

  it("skips items marked disabled", () => {
    const c = one(col({ item: [
      { name: "On", request: { url: "https://x.dev" } },
      { name: "Off", disabled: true, request: { url: "https://x.dev" } },
    ] }));
    expect(c.items.map((i) => i.name)).toEqual(["On"]);
  });

  describe("auth", () => {
    it("collection bearer auth is inherited by every request", () => {
      const c = one(col({
        auth: { type: "bearer", bearer: [{ key: "token", value: "{{token}}" }] },
        item: [{ name: "F", item: [{ name: "R", request: { url: "https://x.dev" } }] }],
      }));
      expect(parsed(route(c.items, "F", "R")).headers).toEqual({ Authorization: "Bearer <<token>>" });
      expect(c.env).toEqual({ token: "" });
    });

    it("basic auth becomes base64(user:pass)", () => {
      const c = one(request({ url: "https://x.dev", auth: { type: "basic", basic: [{ key: "username", value: "ada" }, { key: "password", value: "pä" }] } }));
      expect(parsed(route(c.items, "R")).headers.Authorization).toBe(`Basic ${Buffer.from("ada:pä").toString("base64")}`);
    });

    it("templated basic credentials become a variable to fill in rather than a broken encoding", () => {
      const c = one(request({ url: "https://x.dev", auth: { type: "basic", basic: [{ key: "username", value: "{{user}}" }, { key: "password", value: "{{pass}}" }] } }));
      expect(parsed(route(c.items, "R")).headers.Authorization).toMatch(/^Basic <<BASIC_\w+>>$/);
    });

    it("apikey auth in a header uses its key, defaulting to X-API-Key", () => {
      const named = one(request({ url: "https://x.dev", auth: { type: "apikey", apikey: [{ key: "key", value: "X-Token" }, { key: "value", value: "{{k}}" }] } }));
      expect(parsed(route(named.items, "R")).headers).toEqual({ "X-Token": "<<k>>" });
      const unnamed = one(request({ url: "https://x.dev", auth: { type: "apikey", apikey: [{ key: "value", value: "abc" }] } }));
      expect(parsed(route(unnamed.items, "R")).headers).toEqual({ "X-API-Key": "abc" });
    });

    it("a folder's auth overrides the collection's, and noauth opts a request out", () => {
      const c = one(col({
        auth: { type: "bearer", bearer: [{ key: "token", value: "col" }] },
        item: [
          { name: "F", auth: { type: "bearer", bearer: [{ key: "token", value: "folder" }] }, item: [
            { name: "Inherit", request: { url: "https://x.dev" } },
            { name: "Explicit inherit", request: { url: "https://x.dev", auth: { type: "inherit" } } },
            { name: "None", request: { url: "https://x.dev", auth: { type: "noauth" } } },
            { name: "Own", request: { url: "https://x.dev", auth: { type: "bearer", bearer: [{ key: "token", value: "own" }] } } },
          ] },
          { name: "NoAuthFolder", auth: { type: "noauth" }, item: [{ name: "R", request: { url: "https://x.dev" } }] },
        ],
      }));
      expect(parsed(route(c.items, "F", "Inherit")).headers).toEqual({ Authorization: "Bearer folder" });
      expect(parsed(route(c.items, "F", "Explicit inherit")).headers).toEqual({ Authorization: "Bearer folder" });
      expect(parsed(route(c.items, "F", "None")).headers).toEqual({});
      expect(parsed(route(c.items, "F", "Own")).headers).toEqual({ Authorization: "Bearer own" });
      expect(parsed(route(c.items, "NoAuthFolder", "R")).headers).toEqual({});
    });

    it("an explicit Authorization header wins over the auth preset", () => {
      const c = one(request({ url: "https://x.dev", header: [{ key: "Authorization", value: "Custom 1" }], auth: { type: "bearer", bearer: [{ key: "token", value: "t" }] } }));
      expect(parsed(route(c.items, "R")).headers).toEqual({ Authorization: "Custom 1" });
    });

    it("unsupported auth types (oauth2, aws…) add nothing", () => {
      const c = one(request({ url: "https://x.dev", auth: { type: "oauth2", oauth2: [] } }));
      expect(parsed(route(c.items, "R")).headers).toEqual({});
    });
  });

  it("an array of collections imports each of them", () => {
    const out = parsePostmanCollection([col({ item: [] }), { info: { name: "Two" }, item: [] }, "junk", null]);
    expect(out.map((c) => c.collectionName)).toEqual(["Shop", "Two"]);
  });

  it("hostile or malformed shapes import what they can without throwing", () => {
    const hostile = [
      col({ item: [null, 1, "x", [], { name: "R", request: null }, { name: "Q", request: { url: 42, header: "nope", body: "x", method: 7 } }] }),
      col({ item: "not an array", variable: { key: "x" } }),
      col({ item: [{ name: "F", item: [null, { request: { url: { host: 5, path: null, query: "x" } } }] }] }),
      { info: { name: "x", schema: SCHEMA }, item: [{ name: "Deep", item: [{ item: [{ item: [{ request: { url: "https://x.dev" } }] }] }] }] },
      col({ auth: "bearer", item: [{ name: "A", request: { url: "https://x.dev", auth: { type: "basic", basic: "x" } } }] }),
    ];
    for (const h of hostile) expect(() => parsePostmanCollection(h)).not.toThrow();
    expect(parsePostmanCollection(hostile[0])[0].items.map((i) => i.name)).toEqual(["Q"]);
    expect(parsePostmanCollection(hostile[1])[0].items).toEqual([]);
    for (const v of [null, undefined, 0, "", "x", {}, { info: null }]) expect(parsePostmanCollection(v)).toEqual([]);
  });

  it("a __proto__ variable does not pollute Object.prototype", () => {
    parsePostmanCollection(JSON.parse(`{"info":{"name":"x"},"item":[],"variable":[{"key":"__proto__","value":"polluted"}]}`));
    expect(({} as Record<string, unknown>).polluted).toBeUndefined();
  });
});
