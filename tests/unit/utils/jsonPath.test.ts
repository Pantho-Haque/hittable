import { describe, it, expect } from "vitest";
import { queryJsonPath, parseJsonPath } from "@/utils/jsonPath";

// FR-13: the JSONPath filter's engine. RFC 9535 subset, no eval.

const doc = {
  store: {
    book: [
      { title: "A", price: 8, id: 1 },
      { title: "B", price: 12, id: 2 },
      { title: "C", price: 9, id: 3 },
    ],
    bike: { color: "red", id: 9 },
  },
  "odd key": { "it's": true },
};

const q = (path: string, data: unknown = doc) => {
  const r = queryJsonPath(data, path);
  if (!r.ok) throw new Error(r.error);
  return r;
};

describe("queryJsonPath", () => {
  it("$ is the whole document", () => {
    expect(q("$").matches).toEqual([doc]);
    expect(q("$").paths).toEqual(["$"]);
  });

  it("dot and bracket members", () => {
    expect(q("$.store.bike.color").matches).toEqual(["red"]);
    expect(q("$['store']['bike']").matches).toEqual([doc.store.bike]);
    expect(q('$["odd key"]["it\'s"]').matches).toEqual([true]);
    expect(q("$['odd key']['it\\'s']").paths).toEqual(["$['odd key']['it\\'s']"]);
  });

  it("indexes, negative from the end", () => {
    expect(q("$.store.book[0].title").matches).toEqual(["A"]);
    expect(q("$.store.book[-1].title").matches).toEqual(["C"]);
    expect(q("$.store.book[5]").matches).toEqual([]);
    expect(q("$.store.book[-1]").paths).toEqual(["$['store']['book'][2]"]);
  });

  it("wildcards on arrays and objects", () => {
    expect(q("$.store.book[*].title").matches).toEqual(["A", "B", "C"]);
    expect(q("$.store.bike.*").matches).toEqual(["red", 9]);
    expect(q("$.users[*].name", { users: [{ name: "a" }, { name: "b" }] }).matches).toEqual(["a", "b"]);
  });

  it("slices", () => {
    expect(q("$.store.book[0:2].title").matches).toEqual(["A", "B"]);
    expect(q("$.store.book[1:].title").matches).toEqual(["B", "C"]);
    expect(q("$.store.book[:1].title").matches).toEqual(["A"]);
    expect(q("$.store.book[-2:].title").matches).toEqual(["B", "C"]);
    expect(q("$.store.book[2:1]").matches).toEqual([]);
  });

  it("recursive descent", () => {
    const r = q("$..id");
    expect(r.matches).toEqual([1, 2, 3, 9]);
    expect(r.paths).toContain("$['store']['bike']['id']");
    expect(q("$..book[-1].title").matches).toEqual(["C"]);
    expect(q("$..[0].title").matches).toEqual(["A"]);
    expect(q("$.store.bike..*").matches).toEqual(["red", 9]);
  });

  it("a path that matches nothing is valid with no results", () => {
    expect(q("$.nope").matches).toEqual([]);
    expect(q("$.store.bike.color.length").matches).toEqual([]);
  });

  it.each(["$[", "$..", "foo", "", "$.", "$['a", "$['a'", "$[a]", "$[1.5]", "$[?(@.x)]", "$[]", "$.a b", "$x"])(
    "%j is invalid",
    (path) => {
      const r = queryJsonPath(doc, path);
      expect(r.ok).toBe(false);
      if (!r.ok) expect(r.error).toBeTruthy();
    },
  );

  it("parses into segments without evaluating anything", () => {
    expect(parseJsonPath("$..a[1:3]")).toEqual({
      ok: true,
      segments: [
        { type: "name", name: "a", descendant: true },
        { type: "slice", start: 1, end: 3, descendant: false },
      ],
    });
  });
});
