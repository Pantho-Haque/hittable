import { describe, it, expect, beforeEach } from "vitest";
import { createFolderBackend } from "@/services/workspace/folderBackend";
import { readEnv } from "@/services/workspace/backend";
import { scaffoldHittable } from "@/services/workspace/DirectoryService";
import { FakeDir, FakeFile, asDir } from "../helpers/fakeFs";

// The WorkspaceBackend contract, over a folder on disk.

let root: FakeDir;
const backend = () => createFolderBackend(asDir(root));
const text = (b: Uint8Array) => new TextDecoder().decode(b);

beforeEach(() => {
  root = new FakeDir("proj", { hittable: { "env.json": '{"A":"1"}', api: { "get.hit": "{}" } }, "README.md": "# hi" });
});

describe("folder backend", () => {
  it("is a writable folder workspace labelled with the folder name", () => {
    const b = backend();
    expect([b.kind, b.label, b.readOnly]).toEqual(["folder", "proj", false]);
    expect(b.status).toBeUndefined();
  });

  it("lists the whole tree with root-relative paths", async () => {
    const tree = await backend().listTree();
    expect(tree.map((n) => [n.name, n.kind, n.path])).toEqual([
      ["hittable", "directory", ["hittable"]],
      ["README.md", "file", ["README.md"]],
    ]);
    expect(tree[0].children![1].children![0]).toEqual({ name: "get.hit", path: ["hittable", "api", "get.hit"], kind: "file" });
  });

  it("reads bytes, binary-safe", async () => {
    const png = new FakeFile("logo.png");
    png.bytes = new Uint8Array([0x89, 0x00, 0xff]);
    root.children.set("logo.png", png);
    expect(text(await backend().readFile(["hittable", "api", "get.hit"]))).toBe("{}");
    expect([...(await backend().readFile(["logo.png"]))]).toEqual([0x89, 0x00, 0xff]);
    await expect(backend().readFile(["missing.txt"])).rejects.toThrow();
  });

  it("writes text, creating the file when needed", async () => {
    await backend().writeFile(["README.md"], "# new");
    await backend().writeFile(["hittable", "new", "x.hit"], "{}");
    expect(root.toTree()["README.md"]).toBe("# new");
    expect((root.at("hittable/new/x.hit") as FakeFile).content).toBe("{}");
  });

  it("writes bytes for an upload, creating folders and replacing a file", async () => {
    const png = new Uint8Array([0x89, 0x50, 0, 255]);
    await backend().writeBytes(["hittable", "files", "a.png"], png);
    expect((root.at("hittable/files/a.png") as FakeFile).bytes).toEqual(png);
    await backend().writeBytes(["hittable", "files", "a.png"], png.slice(0, 2));
    expect([...(await backend().readFile(["hittable", "files", "a.png"]))]).toEqual([0x89, 0x50]);
  });

  it("creates files and folders, refusing a name in use", async () => {
    const b = backend();
    await b.createFile(["hittable", "shared", "a.hit"], "A");
    await b.createFile(["empty.txt"]);
    await b.createDirectory(["docs"]);
    expect((root.at("hittable/shared/a.hit") as FakeFile).content).toBe("A");
    expect((root.at("empty.txt") as FakeFile).content).toBe("");
    expect(root.at("docs")!.kind).toBe("directory");
    await expect(b.createFile(["README.md"], "x")).rejects.toThrow('"README.md" already exists');
    await expect(b.createDirectory(["hittable"])).rejects.toThrow("already exists");
    expect(root.toTree()["README.md"]).toBe("# hi");
  });

  it("renames files and folders, also into another folder, refusing a taken name", async () => {
    const b = backend();
    await b.rename(["README.md"], ["INTRO.md"]);
    await b.rename(["hittable", "api"], ["hittable", "v2"]);
    await b.rename(["INTRO.md"], ["hittable", "v2", "INTRO.md"]);
    expect(root.toTree()).toEqual({ hittable: { "env.json": '{"A":"1"}', v2: { "get.hit": "{}", "INTRO.md": "# hi" } } });
    await expect(b.rename(["hittable", "v2", "get.hit"], ["hittable", "v2", "INTRO.md"])).rejects.toThrow("already exists");
    expect((root.at("hittable/v2/get.hit") as FakeFile).content).toBe("{}");
  });

  it("renames a binary file byte for byte", async () => {
    const png = new FakeFile("logo.png");
    png.bytes = new Uint8Array([0x89, 0x50, 0x00, 0xff, 0xfe, 0x80]);
    root.children.set("logo.png", png);
    await backend().rename(["logo.png"], ["icon.png"]);
    expect(root.at("logo.png")).toBeUndefined();
    expect([...((root.at("icon.png") as FakeFile).bytes ?? [])]).toEqual([0x89, 0x50, 0x00, 0xff, 0xfe, 0x80]);
  });

  it("removes files and whole folders", async () => {
    await backend().remove(["hittable", "api"]);
    await backend().remove(["README.md"]);
    expect(root.toTree()).toEqual({ hittable: { "env.json": '{"A":"1"}' } });
  });

  it("env: env.local.json layers over env.json; broken or missing files read as empty", async () => {
    (root.at("hittable") as FakeDir).children.set("env.local.json", new FakeFile("env.local.json", '{"A":"local","B":2}'));
    expect(await readEnv(backend())).toEqual({ A: "local", B: "2" });
    (root.at("hittable/env.json") as FakeFile).content = "[1]";
    (root.at("hittable/env.local.json") as FakeFile).content = "{oops";
    expect(await readEnv(backend())).toEqual({});
    expect(await readEnv(createFolderBackend(asDir(new FakeDir("empty"))))).toEqual({});
  });

  it("scaffolding adds hittable/.gitignore with env.local.json, once", async () => {
    root = new FakeDir("p");
    await scaffoldHittable(asDir(root));
    await scaffoldHittable(asDir(root));
    expect((root.at("hittable/.gitignore") as FakeFile).content).toBe("env.local.json\n");
    expect((root.at("hittable/env.json") as FakeFile).content).toBe("{}");
  });
});
