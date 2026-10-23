// @vitest-environment jsdom
import { useEffect } from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, render, renderHook, waitFor } from "@testing-library/react";
import { WorkspaceProvider, useWorkspace, connectingLabel } from "@/context/workspaceContext";
import { FakeDir, FakeFile, asDir } from "../helpers/fakeFs";

// jsdom has no IndexedDB; the handle store is the boundary, kept in memory.
const handleStore = vi.hoisted(() => ({ handle: null as unknown }));
vi.mock("@/services/workspace/HandleStore", () => ({
  saveDirectoryHandle: vi.fn(async (h: unknown) => { handleStore.handle = h; }),
  loadDirectoryHandle: vi.fn(async () => handleStore.handle),
  removeDirectoryHandle: vi.fn(async () => { handleStore.handle = null; }),
  verifyPermission: vi.fn(async (h: { requestPermission: () => Promise<string> }) => (await h.requestPermission()) === "granted"),
}));

type Ctx = ReturnType<typeof useWorkspace>;

function mount() {
  const ctxRef: { current: Ctx | null } = { current: null };
  function Probe() {
    const ctx = useWorkspace();
    useEffect(() => { ctxRef.current = ctx; });
    return null;
  }
  render(<WorkspaceProvider><Probe /></WorkspaceProvider>);
  return () => ctxRef.current!;
}

let root: FakeDir;

async function openedWorkspace() {
  const ctx = mount();
  await act(() => ctx().openFolder());
  return ctx;
}

beforeEach(() => {
  localStorage.clear();
  handleStore.handle = null;
  root = new FakeDir("my-repo", { "README.md": "# repo", "todo.txt": "ship it" });
  window.showDirectoryPicker = vi.fn(async () => asDir(root)) as unknown as typeof window.showDirectoryPicker;
});
afterEach(() => {
  delete (window as Partial<Window>).showDirectoryPicker;
});

describe("WorkspaceProvider", () => {
  it("useWorkspace outside the provider throws a clear error", () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => renderHook(() => useWorkspace())).toThrow(/within a WorkspaceProvider/);
  });

  it("starts in Local mode", () => {
    const ctx = mount();
    expect(ctx().mode).toBe("local");
    expect(ctx().directoryHandle).toBeNull();
  });

  it("names each connecting stage for the loader", () => {
    expect(connectingLabel("preparing")).toMatch(/Preparing/);
    expect(connectingLabel("scanning")).toMatch(/Reading folder/);
    expect(connectingLabel("reconnecting")).toMatch(/Reconnecting/);
  });

  describe("opening a folder", () => {
    it("scaffolds hittable/ with env.json, a sample note and a runnable test.hit", async () => {
      const ctx = await openedWorkspace();
      const tree = root.toTree();
      expect(tree["README.md"]).toBe("# repo");
      expect(tree["todo.txt"]).toBe("ship it");
      const hittable = tree.hittable as Record<string, unknown>;
      expect(hittable["env.json"]).toBe("{}");
      expect((hittable.notes as Record<string, string>)["sample.md"]).toMatch(/^# Sample Note/);
      const hit = JSON.parse((hittable.testcollection as Record<string, string>)["test.hit"]);
      expect(hit).toMatchObject({ method: "GET", headers: {}, params: {}, body: "", response: null });
      expect(hit.url).toMatch(/^https:\/\//);
      expect(ctx().mode).toBe("directory");
      expect(ctx().connectingStage).toBeNull();
    });

    it("never overwrites files that already exist", async () => {
      root = new FakeDir("repo", {
        hittable: { "env.json": '{"BASE_URL":"https://api.example.com"}', notes: { "mine.md": "keep" }, testcollection: { "test.hit": "custom" } },
      });
      const ctx = await openedWorkspace();
      expect(root.toTree()).toEqual({
        hittable: { "env.json": '{"BASE_URL":"https://api.example.com"}', notes: { "mine.md": "keep" }, testcollection: { "test.hit": "custom" } },
      });
      expect(ctx().envContent).toEqual({ BASE_URL: "https://api.example.com" });
    });

    it("reads the whole folder into the explorer tree", async () => {
      const ctx = await openedWorkspace();
      const names = ctx().tree.map((n) => n.name).sort();
      expect(names).toEqual(["README.md", "hittable", "todo.txt"]);
      const hittable = ctx().tree.find((n) => n.name === "hittable")!;
      expect(hittable.kind).toBe("directory");
      expect(hittable.children!.map((n) => n.name).sort()).toEqual(["env.json", "notes", "testcollection"]);
    });

    it("remembers the choice and the folder handle", async () => {
      await openedWorkspace();
      expect(localStorage.getItem("hittable-workspace-mode")).toBe("directory");
      expect(handleStore.handle).toBe(root);
    });

    it("a cancelled picker stays in Local mode without logging an error", async () => {
      const err = vi.spyOn(console, "error").mockImplementation(() => {});
      window.showDirectoryPicker = vi.fn(async () => { throw new DOMException("cancel", "AbortError"); }) as unknown as typeof window.showDirectoryPicker;
      const ctx = await openedWorkspace();
      expect(ctx().mode).toBe("local");
      expect(ctx().connectingStage).toBeNull();
      expect(err).not.toHaveBeenCalled();
    });
  });

  it("disconnecting returns to Local mode and forgets the folder", async () => {
    const ctx = await openedWorkspace();
    act(() => ctx().disconnectFolder());
    expect(ctx().mode).toBe("local");
    expect(ctx().tree).toEqual([]);
    expect(ctx().directoryHandle).toBeNull();
    expect(localStorage.getItem("hittable-workspace-mode")).toBe("local");
    await waitFor(() => expect(handleStore.handle).toBeNull());
  });

  describe("reconnecting on the next visit", () => {
    it("reopens the saved folder when permission is granted", async () => {
      root = new FakeDir("repo", { hittable: { "env.json": '{"K":"v"}' } });
      handleStore.handle = root;
      localStorage.setItem("hittable-workspace-mode", "directory");
      const ctx = mount();
      await waitFor(() => expect(ctx().mode).toBe("directory"));
      expect(ctx().envContent).toEqual({ K: "v" });
      expect(ctx().directoryHandle).toBe(root);
    });

    it("falls back to Local mode and drops the handle when permission is refused", async () => {
      root.permission = "denied";
      handleStore.handle = root;
      localStorage.setItem("hittable-workspace-mode", "directory");
      const ctx = mount();
      await waitFor(() => expect(handleStore.handle).toBeNull());
      expect(ctx().mode).toBe("local");
    });

    it("does nothing when the last mode was Local", async () => {
      handleStore.handle = root;
      const ctx = mount();
      await act(async () => {});
      expect(ctx().mode).toBe("local");
      expect(handleStore.handle).toBe(root);
    });
  });

  describe("files", () => {
    async function openFile(ctx: () => Ctx, path: string) {
      const handle = root.at(path) as FakeFile;
      act(() => ctx().setActiveFile({ path: path.split("/"), handle: handle as unknown as FileSystemFileHandle }));
      await waitFor(() => expect(ctx().isFileLoaded).toBe(true));
      return handle;
    }

    it("routes files by name: .hit, env.json, .md and everything else", async () => {
      const ctx = await openedWorkspace();
      await openFile(ctx, "hittable/testcollection/test.hit");
      expect(ctx().activeFile!.kind).toBe("hit");
      await openFile(ctx, "hittable/env.json");
      expect(ctx().activeFile!.kind).toBe("env");
      await openFile(ctx, "hittable/notes/sample.md");
      expect(ctx().activeFile!.kind).toBe("markdown");
      await openFile(ctx, "todo.txt");
      expect(ctx().activeFile!.kind).toBe("text");
      expect(ctx().rawTextContent).toBe("ship it");
    });

    it("turns a burst of keystrokes into one write of the latest text, never dropping or reordering", async () => {
      const ctx = await openedWorkspace();
      const file = await openFile(ctx, "README.md");
      let last: Promise<void> | void;
      act(() => {
        ctx().saveRawTextContent("a");
        ctx().saveRawTextContent("ab");
        last = ctx().saveRawTextContent("abc");
      });
      await act(async () => { await last; });
      expect(file.writes).toEqual(["abc"]);
      // A save after that write began is written too.
      act(() => { last = ctx().saveRawTextContent("abcd"); });
      await act(async () => { await last; });
      expect(file.writes).toEqual(["abc", "abcd"]);
      expect(file.content).toBe("abcd");
    });

    it("saving env.json re-reads it so new variables resolve at once", async () => {
      const ctx = await openedWorkspace();
      await openFile(ctx, "hittable/env.json");
      let p: Promise<void> | void;
      act(() => { p = ctx().saveRawTextContent('{"TOKEN":"abc"}'); });
      await act(async () => { await p; });
      expect(ctx().envContent).toEqual({ TOKEN: "abc" });
    });

    it("refreshEnv re-reads env.json changed by another tool", async () => {
      const ctx = await openedWorkspace();
      (root.at("hittable/env.json") as FakeFile).content = '{"FROM":"terminal"}';
      await act(() => ctx().refreshEnv());
      expect(ctx().envContent).toEqual({ FROM: "terminal" });
    });

    it("closing the file clears the editor content", async () => {
      const ctx = await openedWorkspace();
      await openFile(ctx, "README.md");
      act(() => ctx().setActiveFile(null));
      expect(ctx().activeFile).toBeNull();
      expect(ctx().rawTextContent).toBe("");
    });
  });

  describe("explorer operations", () => {
    it("creating a .hit file writes a blank request template", async () => {
      const ctx = await openedWorkspace();
      await act(() => ctx().createEntry(asDir(root.at("hittable") as FakeDir), "users.hit", "file"));
      expect(JSON.parse((root.at("hittable/users.hit") as FakeFile).content)).toEqual({
        method: "GET", url: "", headers: {}, params: {}, body: "", response: null,
      });
      expect(ctx().tree.find((n) => n.name === "hittable")!.children!.some((c) => c.name === "users.hit")).toBe(true);
    });

    it("creating any other file makes it empty; creating a folder makes a folder", async () => {
      const ctx = await openedWorkspace();
      await act(() => ctx().createEntry(asDir(root), "notes.txt", "file"));
      await act(() => ctx().createEntry(asDir(root), "api", "directory"));
      expect((root.at("notes.txt") as FakeFile).content).toBe("");
      expect(root.at("api")!.kind).toBe("directory");
    });

    it("creating a file with a name that already exists fails and keeps the existing file", async () => {
      const ctx = await openedWorkspace();
      await expect(act(() => ctx().createEntry(asDir(root), "README.md", "file"))).rejects.toThrow(/already exists/);
      expect((root.at("README.md") as FakeFile).content).toBe("# repo");
    });

    it("deleting a folder removes it and everything inside", async () => {
      const ctx = await openedWorkspace();
      await act(() => ctx().deleteEntry(asDir(root.at("hittable") as FakeDir), "testcollection"));
      expect(root.at("hittable/testcollection")).toBeUndefined();
      expect(ctx().tree.find((n) => n.name === "hittable")!.children!.map((c) => c.name)).not.toContain("testcollection");
    });

    it("renaming a file keeps its content", async () => {
      const ctx = await openedWorkspace();
      await act(() => ctx().renameEntry(asDir(root), "README.md", "INTRO.md", "file"));
      expect(root.at("README.md")).toBeUndefined();
      expect((root.at("INTRO.md") as FakeFile).content).toBe("# repo");
    });

    it("renaming a folder copies its whole subtree", async () => {
      const ctx = await openedWorkspace();
      await act(() => ctx().renameEntry(asDir(root.at("hittable") as FakeDir), "testcollection", "users", "directory"));
      expect(root.at("hittable/testcollection")).toBeUndefined();
      expect(JSON.parse((root.at("hittable/users/test.hit") as FakeFile).content).method).toBe("GET");
    });

    it("renaming onto an existing name fails and loses nothing", async () => {
      root = new FakeDir("repo", { "a.hit": "A", "b.hit": "B" });
      const ctx = await openedWorkspace();
      await expect(act(() => ctx().renameEntry(asDir(root), "a.hit", "b.hit", "file"))).rejects.toThrow(/already exists/);
      expect((root.at("a.hit") as FakeFile).content).toBe("A");
      expect((root.at("b.hit") as FakeFile).content).toBe("B");
    });
  });
});
