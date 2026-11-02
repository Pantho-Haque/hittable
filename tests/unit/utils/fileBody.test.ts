import { describe, it, expect } from "vitest";
import fs from "node:fs";
import path from "node:path";
import { parseFileRefs, resolveRefPath, relativeRefPath, fileSpecText, guessType, fileSpecsOf } from "@/utils/fileBody";

const ROOT = path.resolve(__dirname, "../../..");
const WEB = path.join(ROOT, "tests/unit/fixtures/fileBody.cases.json");
const GO = path.join(ROOT, "shellapp/internal/hitfile/testdata/filebody_cases.json");
type Case = { name: string; contentType: string; body: string; expect: unknown };
const cases: Case[] = JSON.parse(fs.readFileSync(WEB, "utf8"));

describe("parseFileRefs: the cases shared with the terminal app", () => {
  it("both apps test against the same cases", () => {
    expect(fs.readFileSync(GO, "utf8")).toBe(fs.readFileSync(WEB, "utf8"));
  });

  for (const c of cases) {
    it(c.name, () => {
      if (c.expect && typeof c.expect === "object" && "error" in c.expect) {
        expect(() => parseFileRefs(c.body, c.contentType)).toThrow();
      } else {
        expect(parseFileRefs(c.body, c.contentType)).toEqual(c.expect);
      }
    });
  }
});

describe("file paths", () => {
  const hit = ["hittable", "users", "upload.hit"];
  it("resolve against the .hit file's folder", () => {
    expect(resolveRefPath(hit, "files/a.png")).toEqual(["hittable", "users", "files", "a.png"]);
    expect(resolveRefPath(hit, "./a.png")).toEqual(["hittable", "users", "a.png"]);
    expect(resolveRefPath(hit, "../files/a.png")).toEqual(["hittable", "files", "a.png"]);
    expect(resolveRefPath(hit, "..\\files\\a.png")).toEqual(["hittable", "files", "a.png"]);
    expect(resolveRefPath(hit, "../../README.md")).toEqual(["README.md"]);
  });

  it("never leave the workspace or go absolute", () => {
    expect(() => resolveRefPath(hit, "../../../etc/passwd")).toThrow(/leaves the workspace/);
    for (const p of ["/etc/passwd", "~/a.png", "C:\\x\\a.png", "C:/x/a.png"]) {
      expect(() => resolveRefPath(hit, p)).toThrow(/terminal app/);
    }
  });

  it("are stored relative to the .hit file", () => {
    expect(relativeRefPath(hit, ["hittable", "files", "a.png"])).toBe("../files/a.png");
    expect(relativeRefPath(hit, ["hittable", "users", "files", "a.png"])).toBe("files/a.png");
    expect(relativeRefPath(["req.hit"], ["hittable", "files", "a.png"])).toBe("hittable/files/a.png");
    for (const t of [["hittable", "files", "a.png"], ["README.md"], ["hittable", "users", "x", "y.bin"]]) {
      expect(resolveRefPath(hit, relativeRefPath(hit, t))).toEqual(t);
    }
  });
});

describe("helpers", () => {
  it("writes a file spec back as body text", () => {
    expect(fileSpecText({ path: "a.png" })).toBe("@a.png");
    expect(fileSpecText({ path: "a.png", type: "image/png", filename: "me.png" })).toBe("@a.png;type=image/png;filename=me.png");
    expect(parseFileRefs(fileSpecText({ path: "a b.png", type: "image/png" }), "")).toEqual({ kind: "binary", file: { path: "a b.png", type: "image/png" } });
  });
  it("guesses content types from the extension", () => {
    expect(guessType("a.PNG")).toBe("image/png");
    expect(guessType("report.pdf")).toBe("application/pdf");
    expect(guessType("blob")).toBe("application/octet-stream");
  });
  it("lists every referenced file", () => {
    expect(fileSpecsOf(parseFileRefs('{"a":"1","f":"@x.png","g":"@y.png"}', "multipart/form-data")).map((f) => f.path)).toEqual(["x.png", "y.png"]);
    expect(fileSpecsOf(null)).toEqual([]);
  });
});
