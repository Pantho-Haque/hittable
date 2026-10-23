import { describe, it, expect } from "vitest";
import { parseHitFile, serializeHitFile } from "@/utils/workspace/hitFileParser";
import { parseEnvFile, serializeEnvFile } from "@/utils/workspace/envFileParser";
import {
  buildTree, findNodeByPath, getParentHandle, getParentHandleForPath, isReservedRootEntry, getFileKind,
} from "@/utils/workspace/directoryTreeHelpers";
import {
  fileExists, directoryExists, getFileHandle, getDirectoryHandle, readFileText, writeFileText, createFile,
  createDirectory, removeEntry, renameEntry, listDirectory,
} from "@/utils/workspace/fsHelpers";
import { getFileIcon, getFolderIcon } from "@/utils/workspace/fileIcons";
import { languageForFile, languageNameForFile, languageHasCompletion } from "@/utils/workspace/editorLanguages";
import { FakeDir, FakeFile, asDir } from "../helpers/fakeFs";
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

describe("env.json", () => {
  it("parses a flat object and rejects arrays, scalars and invalid JSON", () => {
    expect(parseEnvFile('{"BASE_URL":"https://x.dev","TOKEN":""}')).toEqual({ BASE_URL: "https://x.dev", TOKEN: "" });
    for (const bad of ["[1]", "1", '"s"', "null", "{bad", ""]) expect(parseEnvFile(bad)).toEqual({});
  });
  it("serialises with two-space indent and a trailing newline", () => {
    expect(serializeEnvFile({ A: "1" })).toBe('{\n  "A": "1"\n}\n');
  });
});

describe("directory tree helpers", () => {
  const root = new FakeDir("root", { hittable: { "env.json": "{}", api: { "get.hit": "{}" }, notes: { "a.md": "# a" } }, "README.md": "" });

  it("builds the tree recursively", async () => {
    const tree = await buildTree(asDir(root));
    expect(tree.map((n) => [n.name, n.kind])).toEqual([["hittable", "directory"], ["README.md", "file"]]);
    expect(findNodeByPath(tree, ["hittable", "api", "get.hit"])?.kind).toBe("file");
    expect(findNodeByPath(tree, ["hittable", "missing"])).toBeNull();
    expect(findNodeByPath(tree, ["README.md", "x"])).toBeNull();
    expect(findNodeByPath(tree, [])).toBeNull();
  });

  it("finds the parent directory handle of a path", async () => {
    const tree = await buildTree(asDir(root));
    expect(getParentHandle(tree, ["hittable", "api", "get.hit"])).toBe(root.at("hittable/api"));
    expect(getParentHandleForPath(tree, ["hittable", "env.json"])).toBe(root.at("hittable"));
    expect(getParentHandle(tree, ["README.md"])).toBeNull();
    expect(getParentHandle(tree, ["README.md", "x"])).toBeNull();
  });

  it("reserves env.json, env and notes", () => {
    expect(["env.json", "env", "notes"].every(isReservedRootEntry)).toBe(true);
    expect(["Env.json", "notes.md", "api"].some(isReservedRootEntry)).toBe(false);
  });

  it("routes files to editors by name", () => {
    expect(["a.hit", "env.json", "env", "x.md", "x.json", "Makefile"].map(getFileKind)).toEqual(["hit", "env", "env", "markdown", "text", "text"]);
  });
});

describe("file-system helpers", () => {
  it("checks existence and returns null for missing handles", async () => {
    const d = asDir(new FakeDir("d", { f: "x", sub: {} }));
    expect(await fileExists(d, "f")).toBe(true);
    expect(await fileExists(d, "sub")).toBe(false);
    expect(await directoryExists(d, "sub")).toBe(true);
    expect(await getFileHandle(d, "nope")).toBeNull();
    expect(await getDirectoryHandle(d, "nope")).toBeNull();
  });

  it("creates, writes, reads and lists", async () => {
    const d = asDir(new FakeDir("d"));
    const h = await createFile(d, "a.hit", "one");
    expect(await readFileText(h)).toBe("one");
    await writeFileText(h, "two");
    expect(await readFileText(h)).toBe("two");
    await createDirectory(d, "sub");
    expect(await listDirectory(d)).toEqual([{ name: "a.hit", kind: "file" }, { name: "sub", kind: "directory" }]);
  });

  it("removes a non-empty folder", async () => {
    const raw = new FakeDir("d", { sub: { inner: { "x.hit": "{}" } } });
    await removeEntry(asDir(raw), "sub");
    expect(raw.toTree()).toEqual({});
  });

  it("renames a file, keeping its content", async () => {
    const raw = new FakeDir("d", { "old.hit": '{"url":"x"}' });
    await renameEntry(asDir(raw), "old.hit", "new.hit", "file");
    expect(raw.toTree()).toEqual({ "new.hit": '{"url":"x"}' });
  });

  it("renames a binary file byte for byte", async () => {
    const raw = new FakeDir("d");
    const png = new FakeFile("logo.png");
    png.bytes = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x00, 0xff, 0xfe, 0x80]);
    raw.children.set("logo.png", png);
    await renameEntry(asDir(raw), "logo.png", "icon.png", "file");
    const copy = raw.at("icon.png") as FakeFile;
    expect(raw.at("logo.png")).toBeUndefined();
    expect([...(copy.bytes ?? [])]).toEqual([0x89, 0x50, 0x4e, 0x47, 0x00, 0xff, 0xfe, 0x80]);
  });

  it("renames a folder with nested content", async () => {
    const raw = new FakeDir("d", { api: { "a.hit": "A", deep: { "b.hit": "B" } } });
    await renameEntry(asDir(raw), "api", "v2", "directory");
    expect(raw.toTree()).toEqual({ v2: { "a.hit": "A", deep: { "b.hit": "B" } } });
  });

  it("refuses to rename onto an existing name and leaves both untouched", async () => {
    const raw = new FakeDir("d", { "a.hit": "A", "b.hit": "B", dir: {} });
    await expect(renameEntry(asDir(raw), "a.hit", "b.hit", "file")).rejects.toThrow('"b.hit" already exists');
    await expect(renameEntry(asDir(raw), "a.hit", "dir", "file")).rejects.toThrow("already exists");
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
