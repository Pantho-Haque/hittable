import { describe, it, expect, vi } from "vitest";
import pako from "pako";
import { compressString, decompressString } from "@/utils/compressString";
import { formatJson, formatWithErrorHandleing } from "@/utils/formatJson";
import { parseStringToJson } from "@/utils/JsonStringParsing";
import { getType, valueMatchesSearch, countMatches, getParamsfromUrl, modifyUrlForNewParams } from "@/utils/responsePanelUtils";
import { formatHtml, findElementSourceLine } from "@/utils/htmlFormatter";
import { cn } from "@/utils/cn";
import type { JsonValue } from "@/types";

describe("Hittable native strings (compressString)", () => {
  it("round-trips any text, including unicode and emoji", () => {
    const text = JSON.stringify({ collectionName: "Zoë 🚀", items: [{ curl: "curl -d 'ü'" }] });
    expect(decompressString(compressString(text))).toBe(text);
  });

  it("is URL-safe: no +, / or = padding", () => {
    const s = compressString("x".repeat(1000) + Math.random());
    expect(s).toMatch(/^[A-Za-z0-9_-]+$/);
  });

  it("matches the documented format: pako.deflate, base64, URL-safe", () => {
    const expected = Buffer.from(pako.deflate(new TextEncoder().encode("hello"))).toString("base64")
      .replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
    expect(compressString("hello")).toBe(expected);
  });

  it("an empty collection string round-trips too", () => {
    expect(decompressString(compressString(""))).toBe("");
  });

  it("a corrupt or truncated string fails with the documented message", () => {
    for (const bad of ["not-a-hittable-string", "!!!", "eJw", compressString("hello").slice(0, -4)]) {
      expect(() => decompressString(bad)).toThrow("Invalid or corrupted import data");
    }
  });
});

describe("formatJson", () => {
  it("pretty-prints valid JSON with tabs and reports no error", () => {
    expect(formatJson('{"a":1,"b":[1,2]}')).toEqual({ output: '{\n\t"a": 1,\n\t"b": [\n\t\t1,\n\t\t2\n\t]\n}', error: null });
  });
  it("stringifies non-string values", () => {
    expect(formatJson({ a: "x" }).output).toBe('{\n\t"a": "x"\n}');
    expect(formatJson(5).output).toBe("5");
  });
  it("returns the text unchanged with the parse error for invalid JSON", () => {
    const r = formatJson('{"a":');
    expect(r.output).toBe('{"a":');
    expect(r.error).toBeTruthy();
  });
  it("treats empty, whitespace, null and undefined as empty with no error", () => {
    for (const v of ["", "   ", null, undefined]) expect(formatJson(v)).toEqual({ output: "", error: null });
  });
  it("reports values that cannot be stringified instead of throwing", () => {
    const cyclic: Record<string, unknown> = {};
    cyclic.self = cyclic;
    expect(formatJson(cyclic).error).toBe("Unable to stringify JSON value");
  });
  it("formatWithErrorHandleing sets and clears the error", () => {
    const setError = vi.fn();
    expect(formatWithErrorHandleing('{"a":1}', setError)).toBe('{\n\t"a": 1\n}');
    expect(setError).toHaveBeenLastCalledWith(null);
    formatWithErrorHandleing("{", setError);
    expect(setError).toHaveBeenLastCalledWith(expect.any(String));
  });
});

describe("parseStringToJson", () => {
  it("parses JSON, empty text, and 'Key: Value' lines", () => {
    expect(parseStringToJson('{"a":"1"}')).toEqual({ a: "1" });
    expect(parseStringToJson("  ")).toEqual({});
    expect(parseStringToJson("Content-Type: text/html\nX-Id: 7\nbroken line")).toEqual({ "Content-Type": "text/html", "X-Id": "7" });
  });
});

describe("response search helpers", () => {
  const data: JsonValue = { users: [{ name: "Ada", admin: true }, { name: "Adam", age: 30 }], note: null };
  it("getType names JSON types", () => {
    expect([null, [], {}, "s", 1, true].map((v) => getType(v as never))).toEqual(["null", "array", "object", "string", "number", "boolean"]);
  });
  it("valueMatchesSearch looks through keys and values case-insensitively", () => {
    expect(valueMatchesSearch(data, "ADA")).toBe(true);
    expect(valueMatchesSearch(data, "admin")).toBe(true);
    expect(valueMatchesSearch(data, "30")).toBe(true);
    expect(valueMatchesSearch(data, "nul")).toBe(true);
    expect(valueMatchesSearch(data, "zzz")).toBe(false);
    expect(valueMatchesSearch(data, "")).toBe(false);
  });
  it("countMatches counts every occurrence in keys and values", () => {
    expect(countMatches(data, "ada")).toBe(2);
    // name, Ada (×2), admin, name, Adam (×2), age
    expect(countMatches(data, "a")).toBe(8);
    expect(countMatches("banana", "an")).toBe(2);
    expect(countMatches(data, "")).toBe(0);
  });
});

describe("URL ↔ Params sync", () => {
  it("reads params from the URL, decoded, ignoring the fragment", () => {
    expect(JSON.parse(getParamsfromUrl("https://x.dev/?q=a%20b&e=x%3Dy&k=1=2#frag"))).toEqual({ q: "a b", e: "x=y", k: "1=2" });
    expect(getParamsfromUrl("https://x.dev")).toBe("{}");
  });
  it("keeps malformed percent-escapes as typed instead of throwing", () => {
    expect(JSON.parse(getParamsfromUrl("https://x.dev/?q=100%"))).toEqual({ q: "100%" });
  });
  it("writes params back into the URL, encoded", () => {
    expect(modifyUrlForNewParams("https://x.dev/s?old=1", '{"q":"a b&c","n":"1"}')).toBe("https://x.dev/s?q=a%20b%26c&n=1");
  });
  it("keeps <<KEY>> templates literal so they still resolve when sending", () => {
    expect(modifyUrlForNewParams("<<BASE>>/s", '{"token":"<<TOKEN>>","q":"a<<b"}')).toBe("<<BASE>>/s?token=<<TOKEN>>&q=a%3C%3Cb");
  });
  it("removes the query when params are emptied, and keeps a #fragment last", () => {
    expect(modifyUrlForNewParams("https://x.dev/?a=1", "{}")).toBe("https://x.dev/");
    expect(modifyUrlForNewParams("https://x.dev/p?a=1#top", '{"b":"2"}')).toBe("https://x.dev/p?b=2#top");
    expect(modifyUrlForNewParams("https://x.dev/p#top", '{"b":"2"}')).toBe("https://x.dev/p?b=2#top");
  });
  it("leaves the URL alone while the params JSON is invalid", () => {
    expect(modifyUrlForNewParams("https://x.dev/?a=1", '{"a":')).toBe("https://x.dev/?a=1");
    expect(modifyUrlForNewParams("https://x.dev/?a=1", "null")).toBe("https://x.dev/?a=1");
  });
  it("params → URL → params is lossless", () => {
    const params = { q: "a b&c=d", emoji: "🚀", empty: "" };
    expect(JSON.parse(getParamsfromUrl(modifyUrlForNewParams("https://x.dev", JSON.stringify(params))))).toEqual(params);
  });
});

describe("HTML source formatting", () => {
  it("puts block elements on their own indented lines", () => {
    const out = formatHtml("<html><body><div><p>Hi</p></div></body></html>");
    const lines = out.split("\n");
    expect(lines.length).toBeGreaterThan(3);
    expect(lines.find((l) => l.includes("<p>"))?.startsWith("  ")).toBe(true);
  });
  it("does not lose text content, comments or script bodies", () => {
    const out = formatHtml('<div><!-- note --><script>var a = "<b>";</script><span>keep me</span></div>');
    for (const s of ["<!-- note -->", "keep me", "var a", "</script>"]) expect(out).toContain(s);
  });
  it("finds the source line of an element by tag and attributes", () => {
    const lines = ["<html>", '<a href="/x" id="one">', '<a href="/y" id="two">'];
    expect(findElementSourceLine("a", { href: "/y", id: "two" }, lines)).toBe(1); // half the attributes match on line 1
    expect(findElementSourceLine("a", {}, lines)).toBe(1);
    expect(findElementSourceLine("img", {}, lines)).toBeNull();
  });
});

describe("cn", () => {
  it("merges Tailwind classes, later wins", () => {
    expect(cn("p-2 text-sm", false && "hidden", "p-4")).toBe("text-sm p-4");
  });
});
