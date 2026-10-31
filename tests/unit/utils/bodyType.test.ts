import { describe, it, expect } from "vitest";
import { entriesToForm, formToEntries, getContentType, headersForBodyType, inferBodyType } from "@/utils/bodyType";

const H = (h: Record<string, string>) => JSON.stringify(h);

describe("inferBodyType", () => {
  it("existing requests open as they were: a JSON body is JSON, with or without a header", () => {
    expect(inferBodyType('{"a":1}', "")).toBe("json");
    expect(inferBodyType("[1,2]", H({ "Content-Type": "application/json" }))).toBe("json");
    expect(inferBodyType('{"a":1}', H({ "content-type": "application/vnd.api+json" }))).toBe("json");
  });

  it("half-typed JSON is still JSON, so it is flagged rather than sent as text", () => {
    expect(inferBodyType("{bad", "")).toBe("json");
  });

  it("the Content-Type decides form and text", () => {
    expect(inferBodyType("a=1&b=2", H({ "Content-Type": "application/x-www-form-urlencoded" }))).toBe("form");
    expect(inferBodyType('{"looks":"json"}', H({ "Content-Type": "text/plain" }))).toBe("text");
    expect(inferBodyType("<x/>", H({ "Content-Type": "application/xml" }))).toBe("text");
  });

  it("a non-JSON body with no header is text; an empty body opens as JSON, ready to type", () => {
    expect(inferBodyType("hello", "")).toBe("text");
    expect(inferBodyType("", "")).toBe("json");
    expect(inferBodyType("   ", "{}")).toBe("json");
    expect(inferBodyType("", JSON.stringify({ "Content-Type": "text/plain" }))).toBe("text");
  });

  it("invalid headers JSON does not break inference", () => {
    expect(inferBodyType('{"a":1}', "{not json")).toBe("json");
    expect(getContentType("{not json")).toBe("");
  });
});

describe("headersForBodyType", () => {
  it("adds the Content-Type when there is none", () => {
    expect(JSON.parse(headersForBodyType("{}", "form"))).toEqual({ "Content-Type": "application/x-www-form-urlencoded" });
    expect(JSON.parse(headersForBodyType("", "json"))).toEqual({ "Content-Type": "application/json" });
  });

  it("replaces one this UI set, keeping the user's header name casing", () => {
    expect(JSON.parse(headersForBodyType(H({ "content-type": "application/json", A: "1" }), "text"))).toEqual({
      A: "1",
      "content-type": "text/plain",
    });
  });

  it("never overrides a Content-Type the user chose", () => {
    const h = H({ "Content-Type": "application/vnd.api+json" });
    expect(headersForBodyType(h, "form")).toBe(h);
  });

  it("none removes an automatic Content-Type", () => {
    expect(JSON.parse(headersForBodyType(H({ "Content-Type": "application/json", X: "y" }), "none"))).toEqual({ X: "y" });
  });

  it("leaves invalid headers alone", () => {
    expect(headersForBodyType("{oops", "json")).toBe("{oops");
  });
});

describe("form bodies", () => {
  it("encode spaces and symbols and skip empty keys", () => {
    expect(entriesToForm([["a", "1"], ["b", "x y"], ["", "skip"], ["c", "&="]])).toBe("a=1&b=x+y&c=%26%3D");
  });

  it("round-trip through entries", () => {
    expect(formToEntries("a=1&b=x+y")).toEqual([["a", "1"], ["b", "x y"]]);
    expect(formToEntries("")).toEqual([["", ""]]);
  });
});
