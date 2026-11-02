import { describe, it, expect } from "vitest";
import { parseHitFile, serializeHitFile } from "@/utils/workspace/hitFileParser";
import { findNode, isReservedPath, type WsNode } from "@/services/workspace/backend";
import { editorKind } from "@/utils/workspace/fileKind";
import {
  fileExists, directoryExists, readFileText, writeFileText, createFile, createDirectory, removeEntry, moveEntry,
} from "@/utils/workspace/fsHelpers";
import { getFileIcon, getFolderIcon } from "@/utils/workspace/fileIcons";
import { languageForFile, languageNameForFile, languageHasCompletion } from "@/utils/workspace/editorLanguages";
import { FakeDir, asDir } from "../helpers/fakeFs";
import type { THitFileContent } from "@/types";

describe(".hit files", () => {
  const hit: THitFileContent = {
    method: "GET", url: "<<BASE_URL>>/users/3", headers: { Authorization: "Bearer <<AUTH_TOKEN>>" },
    params: {}, body: "", response: null,
  };

  it("serialises with the documented key order, two-space indent, no HTML escaping, trailing newline", () => {
    const text = serializeHitFile({ response: null, body: "", params: {}, headers: hit.headers, url: hit.url, method: "GET" });
    expect(text).toBe(`{
  "method": "GET",
  "url": "<<BASE_URL>>/users/3",
  "headers": {
    "Authorization": "Bearer <<AUTH_TOKEN>>"
  },
  "params": {},
  "body": "",
  "response": null
}
`);
  });

  it("parse → serialise is stable", () => {
    const text = serializeHitFile(hit);
    expect(serializeHitFile(parseHitFile(text))).toBe(text);
  });

  it("fills missing fields with defaults", () => {
    expect(parseHitFile('{"url":"https://x.dev"}')).toEqual({ method: "GET", url: "https://x.dev", headers: {}, params: {}, body: "", response: null });
  });

  it("returns an empty request for invalid JSON, null or a non-object", () => {
    const empty = { method: "GET", url: "", headers: {}, params: {}, body: "", response: null };
    for (const bad of ["{oops", "", "null"]) expect(parseHitFile(bad)).toEqual(empty);
  });

  it("drops unknown keys on save", () => {
    expect(serializeHitFile({ ...hit, extra: 1 } as THitFileContent)).not.toContain("extra");
  });
});

describe("workspace paths", () => {
  const tree: WsNode[] = [
    { name: "hittable", path: ["hittable"], kind: "directory", children: [
      { name: "api", path: ["hittable", "api"], kind: "directory", children: [{ name: "get.hit", path: ["hittable", "api", "get.hit"], kind: "file" }] },
    ] },
    { name: "README.md", path: ["README.md"], kind: "file" },
  ];

  it("finds a node by path", () => {
    expect(findNode(tree, ["hittable", "api", "get.hit"])?.kind).toBe("file");
    expect(findNode(tree, ["hittable", "missing"])).toBeNull();
    expect(findNode(tree, ["README.md", "x"])).toBeNull();
    expect(findNode(tree, [])).toBeNull();
  });

  it("reserves env.json, env and notes at the root and directly in hittable/", () => {
    expect([["env.json"], ["notes"], ["hittable", "env"], ["hittable", "notes"], ["hittable", "env.json"]].every(isReservedPath)).toBe(true);
    expect([["Env.json"], ["hittable", "notes.md"], ["hittable", "api", "env.json"], ["hittable", "env.local.json"]].some(isReservedPath)).toBe(false);
  });

  it("routes files to editors by name", () => {
    expect(["a.hit", "env.json", "env", "x.md", "x.json", "Makefile"].map(editorKind)).toEqual(["hit", "env", "env", "markdown", "text", "text"]);
  });
});

describe("file-system helpers", () => {
  it("checks existence", async () => {
    const d = asDir(new FakeDir("d", { f: "x", sub: {} }));
    expect(await fileExists(d, "f")).toBe(true);
    expect(await fileExists(d, "sub")).toBe(false);
    expect(await directoryExists(d, "sub")).toBe(true);
    expect(await directoryExists(d, "nope")).toBe(false);
  });

  it("creates, writes and reads", async () => {
    const raw = new FakeDir("d");
    const h = await createFile(asDir(raw), "a.hit", "one");
    expect(await readFileText(h)).toBe("one");
    await writeFileText(h, "two");
    expect(await readFileText(h)).toBe("two");
    await createDirectory(asDir(raw), "sub");
    expect(raw.toTree()).toEqual({ "a.hit": "two", sub: {} });
  });

  it("removes a non-empty folder", async () => {
    const raw = new FakeDir("d", { sub: { inner: { "x.hit": "{}" } } });
    await removeEntry(asDir(raw), "sub");
    expect(raw.toTree()).toEqual({});
  });

  it("moves a folder with nested content into another folder", async () => {
    const raw = new FakeDir("d", { api: { "a.hit": "A", deep: { "b.hit": "B" } }, out: {} });
    await moveEntry(asDir(raw), "api", asDir(raw.at("out") as FakeDir), "v2");
    expect(raw.toTree()).toEqual({ out: { v2: { "a.hit": "A", deep: { "b.hit": "B" } } } });
  });

  it("refuses to move onto an existing name and leaves both untouched", async () => {
    const raw = new FakeDir("d", { "a.hit": "A", "b.hit": "B", dir: {} });
    await expect(moveEntry(asDir(raw), "a.hit", asDir(raw), "b.hit")).rejects.toThrow('"b.hit" already exists');
    await expect(moveEntry(asDir(raw), "a.hit", asDir(raw), "dir")).rejects.toThrow("already exists");
    expect(raw.toTree()).toEqual({ "a.hit": "A", "b.hit": "B", dir: {} });
  });
});

describe("file icons", () => {
  it("prefers exact names over extensions", () => {
    expect(getFileIcon("package.json")).not.toBe(getFileIcon("data.json"));
    expect(getFileIcon("PACKAGE.JSON")).toBe(getFileIcon("package.json"));
  });
  it("collapses .env.* and Dockerfile.* variants onto their base name", () => {
    expect(getFileIcon(".env.local")).toBe(getFileIcon(".env"));
    expect(getFileIcon("Dockerfile.prod")).toBe(getFileIcon("Dockerfile"));
  });
  it("tries compound extensions, falls back to a default", () => {
    expect(getFileIcon("archive.tar.gz")).toBe(getFileIcon("x.gz"));
    expect(getFileIcon("env.d.ts")).toBe(getFileIcon("x.ts"));
    expect(getFileIcon("noext")).toBe(getFileIcon("weird.qqq"));
    expect(getFileIcon("a.hit")).not.toBe(getFileIcon("weird.qqq"));
  });
  it("special folders keep their glyph whether open or not; others switch", () => {
    expect(getFolderIcon("notes", true)).toBe(getFolderIcon("Notes", false));
    expect(getFolderIcon("api", true).Icon).not.toBe(getFolderIcon("api", false).Icon);
  });
});

describe("editor languages", () => {
  it("maps files to the documented grammars", () => {
    const names = ["a.ts", "a.tsx", "a.js", "a.jsx", "a.json", "x.hit", "env", "README.md", "i.html", "s.scss", "i.svg", "c.yml",
      "m.py", "m.go", "m.rs", "q.sql", "run.sh", "Makefile", ".zshrc", "c.toml", "c.ini", "Dockerfile", "r.rb", "l.lua", "c.cpp",
      "J.java", "C.cs", "s.swift", "p.diff"].map(languageNameForFile);
    expect(names).toEqual(["TypeScript", "TSX", "JavaScript", "JSX", "JSON", "JSON", "JSON", "Markdown", "HTML", "CSS", "XML", "YAML",
      "Python", "Go", "Rust", "SQL", "Shell", "Shell", "Shell", "TOML", "INI", "Dockerfile", "Ruby", "Lua", "C/C++",
      "Java", "C#", "Swift", "Diff"]);
  });
  it("unknown files are plain text", () => {
    expect(languageForFile("notes.qqq")).toBeNull();
    expect(languageNameForFile("notes.qqq")).toBe("Plain Text");
  });
  it("only grammars with their own completion opt out of buffer-word completion", () => {
    expect(["a.ts", "a.py", "q.sql", "i.html"].every(languageHasCompletion)).toBe(true);
    expect(["a.json", "a.go", "x.qqq"].some(languageHasCompletion)).toBe(false);
  });
  it("every grammar actually loads", async () => {
    for (const f of ["a.ts", "a.json", "a.md", "a.html", "a.css", "a.xml", "a.yml", "a.py", "a.go", "a.rs", "a.sql", "a.sh",
      "a.toml", "a.ini", "Dockerfile", "a.rb", "a.lua", "a.c", "a.java", "a.cs", "a.swift", "a.diff"]) {
      await expect(languageForFile(f)!.load()).resolves.toBeTruthy();
    }
  });
});
