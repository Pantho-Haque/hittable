import { describe, it, expect } from "vitest";
import { exportToPostmanCollection } from "@/utils/importers/postmanExporter";
import { exportToInsomniaCollection } from "@/utils/importers/insomniaExporter";
import { parsePostmanCollection, isPostmanCollection } from "@/utils/importers/postmanImporter";
import { parseInsomniaExport, isInsomniaExport } from "@/utils/importers/insomniaImporter";
import {
  extractVariableNames, ensureEnvVarsForText, fromPostman, fromInsomnia, toPostman, toInsomnia,
} from "@/utils/importers/variableSyntax";
import { curlConverter, jsonToCurl } from "@/utils/curlConverter";
import type { THittableCollection, THittableItem } from "@/types";

const curl = (method: string, url: string, headers: Record<string, string> = {}, body = "") =>
  jsonToCurl({ method, url, headers: JSON.stringify(headers), body, params: "{}" });

const collection: THittableCollection = {
  collectionName: "Shop",
  env: { BASE_URL: "https://api.shop.dev", TEAM: "core" },
  secrets: { TOKEN: "super-secret-token", API_KEY: "sk_live_123" },
  items: [
    { type: "route", name: "Health", curl: curl("GET", "<<BASE_URL>>/health"), response: '{"status":200}' },
    {
      type: "folder", name: "Users", items: [
        { type: "route", name: "List", curl: curl("GET", "<<BASE_URL>>/users?team=<<TEAM>>", { Authorization: "Bearer <<TOKEN>>" }), response: "" },
        { type: "route", name: "Create", curl: curl("POST", "<<BASE_URL>>/users", { "Content-Type": "application/json", "X-Key": "<<API_KEY>>" }, '{"name":"Zoë","admin":false,"n":1}'), response: "" },
        { type: "folder", name: "Admin", items: [
          { type: "route", name: "Head", curl: curl("HEAD", "<<BASE_URL>>/admin"), response: "" },
          { type: "route", name: "Quote", curl: curl("PUT", "<<BASE_URL>>/q", { X: `say "hi" it's` }, `{"q":"it's"}`), response: "" },
        ] },
      ],
    },
    { type: "folder", name: "Empty", items: [] },
  ],
};

// What the user cares about after a round trip: same tree, same requests.
function shape(items: THittableItem[]): unknown[] {
  return items.map((i) => {
    if (i.type === "folder") return { folder: i.name, items: shape(i.items) };
    const c = curlConverter(i.curl);
    let body: unknown = c.body;
    try { body = JSON.parse(c.body); } catch { /* raw */ }
    return { route: i.name, method: c.method, url: c.url, headers: JSON.parse(c.headers), body };
  });
}

describe("variable syntax", () => {
  it("translates between Hittable, Postman and Insomnia syntax", () => {
    expect(fromPostman("{{a}}/{{ b }}/{{c.d}}/{{e-f}}")).toBe("<<a>>/<<b>>/<<c.d>>/<<e-f>>");
    expect(fromInsomnia("{{ _.a }}/{{_.b}}/{{c}}")).toBe("<<a>>/<<b>>/<<c>>");
    expect(toPostman("<<A>>/<<b_2>> <<not valid>>")).toBe("{{A}}/{{b_2}} <<not valid>>");
    expect(toInsomnia("<<A>>")).toBe("{{ _.A }}");
  });

  it("extractVariableNames finds Postman and Insomnia references once each", () => {
    expect([...extractVariableNames("{{a}} {{ _.b }} {{a}} <<c>>")]).toEqual(["a", "b"]);
  });

  it("ensureEnvVarsForText adds empty entries but never overwrites values", () => {
    const env = { a: "keep" };
    expect(ensureEnvVarsForText("{{a}} {{ _.b }}", env)).toEqual({ a: "keep", b: "" });
    expect(env).toEqual({ a: "keep" });
  });
});

describe("Postman export", () => {
  const out = JSON.parse(exportToPostmanCollection(collection));

  it("is a v2.1 collection that the importer recognises", () => {
    expect(out.info).toEqual({ name: "Shop", schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" });
    expect(isPostmanCollection(out)).toBe(true);
  });

  it("translates <<KEY>> to {{KEY}} instead of inlining values", () => {
    const list = out.item[1].item[0].request;
    expect(list.url).toBe("{{BASE_URL}}/users?team={{TEAM}}");
    expect(list.header).toEqual([{ key: "Authorization", value: "Bearer {{TOKEN}}" }]);
  });

  it("exports env values as variables and secrets by name only, never their values", () => {
    const text = exportToPostmanCollection(collection);
    expect(out.variable).toEqual([
      { key: "BASE_URL", value: "https://api.shop.dev", type: "string" },
      { key: "TEAM", value: "core", type: "string" },
      { key: "TOKEN", value: "", type: "string" },
      { key: "API_KEY", value: "", type: "string" },
    ]);
    expect(text).not.toContain("super-secret-token");
    expect(text).not.toContain("sk_live_123");
  });

  it("puts bodies in raw mode only where the method allows one", () => {
    expect(out.item[1].item[1].request.body).toEqual({ mode: "raw", raw: '{\n\t"name": "Zoë",\n\t"admin": false,\n\t"n": 1\n}' });
    expect(out.item[0].request.body).toBeUndefined();
    expect(out.item[1].item[2].item[0].request).toMatchObject({ method: "HEAD" });
  });

  it("never exports saved responses", () => {
    expect(exportToPostmanCollection(collection)).not.toContain('\\"status\\":200');
  });

  it("round-trips through the importer with the same tree, requests and variables", () => {
    const [back] = parsePostmanCollection(out);
    expect(back.collectionName).toBe("Shop");
    expect(shape(back.items)).toEqual(shape(collection.items));
    expect(back.env).toEqual({ BASE_URL: "https://api.shop.dev", TEAM: "core", TOKEN: "", API_KEY: "" });
  });

  it("copes with a collection missing env, secrets or items", () => {
    const bare = JSON.parse(exportToPostmanCollection({ collectionName: "x" } as THittableCollection));
    expect(bare).toMatchObject({ item: [], variable: [] });
  });
});

describe("Insomnia export", () => {
  const out = JSON.parse(exportToInsomniaCollection(collection));
  const byType = (t: string) => out.resources.filter((r: { _type: string }) => r._type === t);

  it("is a v4 export envelope that the importer recognises", () => {
    expect(out).toMatchObject({ _type: "export", __export_format: 4 });
    expect(isInsomniaExport(out)).toBe(true);
  });

  it("has one workspace, folders as request groups, and unique ids", () => {
    expect(byType("workspace")).toHaveLength(1);
    expect(byType("request_group").map((g: { name: string }) => g.name)).toEqual(["Users", "Admin", "Empty"]);
    const ids = out.resources.map((r: { _id: string }) => r._id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("translates <<KEY>> to {{ _.KEY }} and writes v4 header arrays", () => {
    const list = byType("request").find((r: { name: string }) => r.name === "List");
    expect(list.url).toBe("{{ _.BASE_URL }}/users?team={{ _.TEAM }}");
    expect(list.headers).toEqual([{ name: "Authorization", value: "Bearer {{ _.TOKEN }}" }]);
    expect(list.body).toEqual({});
  });

  it("uses the request's Content-Type as the body mime type", () => {
    const create = byType("request").find((r: { name: string }) => r.name === "Create");
    expect(create.body.mimeType).toBe("application/json");
    expect(JSON.parse(create.body.text)).toEqual({ name: "Zoë", admin: false, n: 1 });
  });

  it("keeps secret names in the base environment but never their values", () => {
    expect(byType("environment")[0].data).toEqual({ BASE_URL: "https://api.shop.dev", TEAM: "core", TOKEN: "", API_KEY: "" });
    expect(exportToInsomniaCollection(collection)).not.toContain("super-secret-token");
  });

  it("guesses the mime type of a body sent without Content-Type", () => {
    const quote = byType("request").find((r: { name: string }) => r.name === "Quote");
    expect(quote.body.mimeType).toBe("application/json");
    const text = JSON.parse(exportToInsomniaCollection({ ...collection, items: [
      { type: "route", name: "T", curl: curl("POST", "https://x.dev", {}, "plain words"), response: "" },
    ] })).resources.find((r: { _type: string }) => r._type === "request");
    expect(text.body).toEqual({ mimeType: "text/plain", text: "plain words" });
  });

  it("round-trips through the importer with the same tree, requests and variables", () => {
    const imported = parseInsomniaExport(out);
    expect(imported).toHaveLength(1);
    expect(imported[0].collectionName).toBe("Shop");
    // The one difference: a body that had no Content-Type comes back declaring
    // the mime type Insomnia needed.
    const expected = shape(collection.items) as { items: { items: { route: string; headers: object }[] }[] }[];
    const quote = expected[1].items[2].items[1];
    expect(quote.route).toBe("Quote");
    quote.headers = { ...quote.headers, "Content-Type": "application/json" };
    expect(shape(imported[0].items)).toEqual(expected);
    expect(imported[0].env).toEqual({ BASE_URL: "https://api.shop.dev", TEAM: "core", TOKEN: "", API_KEY: "" });
  });
});

describe("cross-tool round trip", () => {
  it("Postman → Hittable → Insomnia → Hittable keeps every request", () => {
    const [fromPm] = parsePostmanCollection({
      info: { name: "PM", schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" },
      variable: [{ key: "base", value: "https://x.dev" }],
      item: [{ name: "F", item: [{ name: "Create", request: {
        method: "POST", url: "{{base}}/u", header: [{ key: "A", value: "{{tok}}" }, { key: "Content-Type", value: "application/json" }], body: { mode: "raw", raw: '{"a":1}' },
      } }] }],
    });
    const [again] = parseInsomniaExport(JSON.parse(exportToInsomniaCollection(fromPm)));
    expect(shape(again.items)).toEqual(shape(fromPm.items));
    expect(again.env).toEqual({ base: "https://x.dev", tok: "" });
  });
});
