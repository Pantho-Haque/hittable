import { describe, it, expect } from "vitest";
import { diffJson, diffHeaders } from "@/utils/jsonDiff";

// FR-14: comparing a response with the previous one.

describe("diffJson", () => {
  it("lists exactly a changed, an added and a removed field", () => {
    const a = { id: 1, name: "Ada", old: true };
    const b = { id: 1, name: "Grace", fresh: 1 };
    expect(diffJson(a, b)).toEqual([
      { path: "$['fresh']", kind: "added", after: 1 },
      { path: "$['name']", kind: "changed", before: "Ada", after: "Grace" },
      { path: "$['old']", kind: "removed", before: true },
    ]);
  });

  it("key order is not a change", () => {
    expect(diffJson({ a: 1, b: { c: 2, d: 3 } }, { b: { d: 3, c: 2 }, a: 1 })).toEqual([]);
  });

  it("nested objects and arrays by index", () => {
    expect(diffJson({ u: { tags: ["x", "y"] } }, { u: { tags: ["x", "z", "w"] } })).toEqual([
      { path: "$['u']['tags'][1]", kind: "changed", before: "y", after: "z" },
      { path: "$['u']['tags'][2]", kind: "added", after: "w" },
    ]);
    expect(diffJson([1, 2], [1])).toEqual([{ path: "$[1]", kind: "removed", before: 2 }]);
  });

  it("a type change is one change at that path", () => {
    expect(diffJson({ a: [1] }, { a: { 0: 1 } })).toEqual([
      { path: "$['a']", kind: "changed", before: [1], after: { 0: 1 } },
    ]);
    expect(diffJson(1, "1")).toEqual([{ path: "$", kind: "changed", before: 1, after: "1" }]);
    expect(diffJson(null, null)).toEqual([]);
  });

  it("ignore paths, including everything under them and wildcards", () => {
    const a = { updatedAt: 1, meta: { t: 1, u: 2 }, items: [{ id: 1, v: 1 }, { id: 2, v: 2 }] };
    const b = { updatedAt: 2, meta: { t: 9 }, items: [{ id: 7, v: 1 }, { id: 8, v: 3 }] };
    expect(diffJson(a, b, { ignore: ["$.updatedAt", "$.meta", "$.items[*].id"] })).toEqual([
      { path: "$['items'][1]['v']", kind: "changed", before: 2, after: 3 },
    ]);
    expect(diffJson(a, b, { ignore: ["$..id", "$..v", "$.updatedAt", "$['meta']"] })).toEqual([]);
    // A pattern that does not parse is skipped, not fatal.
    expect(diffJson({ a: 1 }, { a: 2 }, { ignore: ["$["] })).toHaveLength(1);
  });

  it("ignoring an added or removed key", () => {
    expect(diffJson({ a: 1 }, { b: 1 }, { ignore: ["$.a", "$.b"] })).toEqual([]);
  });
});

describe("diffHeaders", () => {
  it("compares names case-insensitively", () => {
    expect(diffHeaders({ "Content-Type": "a", ETag: "1", Old: "x" }, { "content-type": "a", etag: "2", "X-New": "y" })).toEqual([
      { name: "etag", kind: "changed", before: "1", after: "2" },
      { name: "old", kind: "removed", before: "x" },
      { name: "x-new", kind: "added", after: "y" },
    ]);
  });

  it("skips ignored names and tolerates missing headers", () => {
    expect(diffHeaders({ Date: "1" }, { date: "2" }, ["DATE"])).toEqual([]);
    expect(diffHeaders(undefined, null)).toEqual([]);
  });
});
