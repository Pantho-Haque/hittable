import { describe, it, expect } from "vitest";
import { parseInsomniaExport, isInsomniaExport } from "@/utils/importers/insomniaImporter";
import { curlConverter } from "@/utils/curlConverter";
import type { THittableItem, THittableRoute } from "@/types";

const exp = (resources: unknown[]) => ({ _type: "export", __export_format: 4, __export_source: "insomnia.desktop.app:v8", resources });
const ws = { _type: "workspace", _id: "wrk_1", parentId: null, name: "Shop API" };
const reqRes = (over: Record<string, unknown>) => ({ _type: "request", _id: "req_1", parentId: "wrk_1", name: "R", method: "GET", url: "https://x.dev", ...over });

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
const single = (r: Record<string, unknown>) => {
  const [c] = parseInsomniaExport(exp([ws, reqRes(r)]));
  return parsed(route(c.items, String(r.name ?? "R")));
};

describe("isInsomniaExport", () => {
  it("recognises the v4 envelope, a resources array, and a bare resource array", () => {
    expect(isInsomniaExport({ _type: "export" })).toBe(true);
    expect(isInsomniaExport({ __export_format: 4 })).toBe(true);
    expect(isInsomniaExport({ resources: [{ _type: "request" }] })).toBe(true);
    expect(isInsomniaExport([{ _type: "workspace" }, { _type: "request" }])).toBe(true);
  });
  it("rejects Postman, Hittable and junk", () => {
    for (const v of [null, 1, "x", [], [1], [{}], {}, { resources: [] }, { resources: [null] }, { info: { name: "x" }, item: [] }]) {
      expect(isInsomniaExport(v)).toBe(false);
    }
  });
});

describe("parseInsomniaExport", () => {
  it("a workspace becomes a collection and request groups become nested folders", () => {
    const [c, ...rest] = parseInsomniaExport(exp([
      ws,
      { _type: "request_group", _id: "fld_users", parentId: "wrk_1", name: "Users" },
      { _type: "request_group", _id: "fld_admin", parentId: "fld_users", name: "Admin" },
      reqRes({ _id: "r1", parentId: "fld_users", name: "List" }),
      reqRes({ _id: "r2", parentId: "fld_admin", name: "Ban", method: "post" }),
      reqRes({ _id: "r3", parentId: "wrk_1", name: "Health" }),
      { _type: "cookie_jar", _id: "jar", parentId: "wrk_1" },
      { _type: "api_spec", _id: "spec", parentId: "wrk_1" },
    ]));
    expect(rest).toEqual([]);
    expect(c.collectionName).toBe("Shop API");
    // top-level requests are kept alongside folders
    expect(c.items.map((i) => [i.type, i.name])).toEqual([["folder", "Users"], ["route", "Health"]]);
    expect(parsed(route(c.items, "Users", "Admin", "Ban")).method).toBe("POST");
    expect(route(c.items, "Users", "List").type).toBe("route");
  });

  it("translates {{ _.var }} and {{var}} to <<var>> everywhere and creates the referenced variables", () => {
    const [c] = parseInsomniaExport(exp([ws, reqRes({
      method: "POST",
      url: "{{ _.base_url }}/users/{{id}}",
      headers: [{ name: "Authorization", value: "Bearer {{_.token}}" }],
      body: { mimeType: "application/json", text: '{"name":"{{ _.name }}"}' },
    })]));
    const p = parsed(route(c.items, "R"));
    expect(p.url).toBe("<<base_url>>/users/<<id>>");
    expect(p.headers).toEqual({ Authorization: "Bearer <<token>>", "Content-Type": "application/json" });
    expect(JSON.parse(p.body)).toEqual({ name: "<<name>>" });
    expect(c.env).toEqual({ base_url: "", id: "", token: "", name: "" });
  });

  it("reads v4 header arrays, skipping disabled rows, and plain header objects", () => {
    expect(single({ headers: [{ name: "A", value: "1" }, { name: "B", value: "2", disabled: true }, { value: "nameless" }] }).headers).toEqual({ A: "1" });
    expect(single({ headers: { A: "1", B: "{{ _.b }}" } }).headers).toEqual({ A: "1", B: "<<b>>" });
  });

  it("appends enabled parameters to the URL query", () => {
    const p = single({ url: "https://x.dev/s?x=1", parameters: [{ name: "q", value: "a b" }, { name: "off", value: "1", disabled: true }, { name: "t", value: "{{ _.t }}" }] });
    expect(p.url).toBe("https://x.dev/s?x=1&q=a%20b&t=<<t>>");
    expect(p.params).toEqual({ x: "1", q: "a b", t: "<<t>>" });
  });

  it("form body params become a JSON object, with files as @filename", () => {
    const p = single({ method: "POST", body: { mimeType: "multipart/form-data", params: [
      { name: "a", value: "1" }, { name: "f", fileName: "/tmp/x.png" }, { name: "off", value: "x", disabled: true },
    ] } });
    expect(JSON.parse(p.body)).toEqual({ a: "1", f: "@/tmp/x.png" });
    expect(p.headers["Content-Type"]).toBe("multipart/form-data");
  });

  it("an explicit Content-Type header is not overwritten by the body mime type", () => {
    const p = single({ method: "POST", headers: [{ name: "Content-Type", value: "text/plain" }], body: { mimeType: "application/json", text: "{}" } });
    expect(p.headers["Content-Type"]).toBe("text/plain");
  });

  it("maps bearer, basic and apikey authentication to headers", () => {
    expect(single({ authentication: { type: "bearer", token: "{{ _.t }}" } }).headers).toEqual({ Authorization: "Bearer <<t>>" });
    expect(single({ authentication: { type: "bearer", token: "x", prefix: "Token" } }).headers).toEqual({ Authorization: "Token x" });
    expect(single({ authentication: { type: "basic", username: "ada", password: "pw" } }).headers).toEqual({
      Authorization: `Basic ${Buffer.from("ada:pw").toString("base64")}`,
    });
    expect(single({ authentication: { type: "apikey", key: "X-Key", value: "v" } }).headers).toEqual({ "X-Key": "v" });
    expect(single({ authentication: { type: "bearer", token: "x", disabled: true } }).headers).toEqual({});
    expect(single({ authentication: { type: "oauth2" } }).headers).toEqual({});
  });

  it("does not attach a body to GET, HEAD or DELETE", () => {
    for (const method of ["GET", "HEAD", "DELETE"]) {
      const [c] = parseInsomniaExport(exp([ws, reqRes({ method, body: { mimeType: "application/json", text: '{"a":1}' } })]));
      expect(route(c.items, "R").curl).not.toContain("-d");
    }
  });

  it("keeps quotes inside header values and bodies intact", () => {
    const p = single({ method: "PUT", headers: [{ name: "X", value: `say "hi" it's` }], body: { text: "it's raw" } });
    expect(p.headers).toEqual({ X: `say "hi" it's` });
    expect(p.body).toBe("it's raw");
  });

  it("collects environment data, v5 _kvPairData, and skips disabled pairs and non-scalar values", () => {
    const [c] = parseInsomniaExport(exp([
      ws,
      { _type: "environment", _id: "env_1", parentId: "wrk_1", data: { base_url: "https://x.dev", port: 8080, nested: { a: 1 } } },
      { _type: "environment", _id: "env_2", parentId: "env_1", _kvPairData: [{ name: "token", value: "abc", enabled: true }, { name: "off", value: "x", enabled: false }] },
      reqRes({}),
    ]));
    expect(c.env).toEqual({ base_url: "https://x.dev", port: "8080", token: "abc" });
  });

  it("each workspace in one export becomes its own collection", () => {
    const out = parseInsomniaExport(exp([
      ws, { _type: "workspace", _id: "wrk_2", name: "Billing" },
      reqRes({ _id: "a", parentId: "wrk_1", name: "A" }), reqRes({ _id: "b", parentId: "wrk_2", name: "B" }),
    ]));
    expect(out.map((c) => [c.collectionName, c.items.map((i) => i.name)])).toEqual([["Shop API", ["A"]], ["Billing", ["B"]]]);
  });

  it("a bare resource array without a workspace imports into one collection", () => {
    const out = parseInsomniaExport([
      { _type: "request_group", _id: "g", parentId: "__WORKSPACE_ID__", name: "G" },
      reqRes({ _id: "a", parentId: "g", name: "A" }),
      reqRes({ _id: "b", parentId: "__WORKSPACE_ID__", name: "B" }),
    ]);
    expect(out).toHaveLength(1);
    expect(out[0].collectionName).toBe("Imported Collection");
    expect(out[0].items.map((i) => i.name)).toEqual(["G", "B"]);
  });

  it("an empty workspace imports nothing", () => {
    expect(parseInsomniaExport(exp([ws]))).toEqual([]);
    expect(parseInsomniaExport(exp([]))).toEqual([]);
  });

  it("survives a parentId loop without recursing forever", () => {
    const out = parseInsomniaExport(exp([
      ws,
      { _type: "request_group", _id: "a", parentId: "wrk_1", name: "A" },
      { _type: "request_group", _id: "b", parentId: "a", name: "B" },
      { _type: "request_group", _id: "c", parentId: "c", name: "Self" },
      reqRes({ _id: "r", parentId: "b", name: "R" }),
    ]));
    expect(route(out[0].items, "A", "B", "R").type).toBe("route");
  });

  it("hostile or malformed shapes do not throw", () => {
    const hostile = [
      null, 1, "x", {}, { resources: "x" }, { resources: [null, 1, "x"] },
      exp([ws, { _type: "request", _id: "x", parentId: "wrk_1", url: 5, method: 3, headers: "x", parameters: {}, body: "x", authentication: "x" }]),
      exp([ws, { _type: "request", parentId: "wrk_1", headers: [null, 1], parameters: [null], body: { params: [null] } }]),
      exp([{ _type: "environment", data: "x", _kvPairData: "x" }]),
    ];
    for (const h of hostile) expect(() => parseInsomniaExport(h)).not.toThrow();
  });
});
