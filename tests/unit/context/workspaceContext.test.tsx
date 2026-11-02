// @vitest-environment jsdom
import { useEffect } from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, render, renderHook, waitFor } from "@testing-library/react";
import { WorkspaceProvider, useWorkspace, connectingLabel } from "@/context/workspaceContext";
import { NotificationContext } from "@/context/notifyContext";
import { createFolderBackend } from "@/services/workspace/folderBackend";
import type { WorkspaceBackend } from "@/services/workspace/backend";
import { openTool } from "@/utils/uiEvents";
import { FakeDir, FakeFile, asDir } from "../helpers/fakeFs";

// jsdom has no IndexedDB; the handle store is the boundary, kept in memory.
const handleStore = vi.hoisted(() => ({ handle: null as unknown }));
vi.mock("@/services/workspace/HandleStore", () => ({
  saveDirectoryHandle: vi.fn(async (h: unknown) => { handleStore.handle = h; }),
  loadDirectoryHandle: vi.fn(async () => handleStore.handle),
  removeDirectoryHandle: vi.fn(async () => { handleStore.handle = null; }),
  verifyPermission: vi.fn(async (h: FakeDir, prompt = true) =>
    (await h.queryPermission()) === "granted" || (prompt && (await h.requestPermission()) === "granted")),
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

  it("starts with no workspace open", () => {
    const ctx = mount();
    expect(ctx().backendKind).toBeNull();
    expect(ctx().tree).toEqual([]);
    expect(ctx().status).toEqual({});
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
      expect(ctx().backendKind).toBe("folder");
      expect(ctx().label).toBe("my-repo");
      expect(ctx().readOnly).toBe(false);
      expect(ctx().status).toEqual({});
      expect(ctx().connectingStage).toBeNull();
    });

    it("never overwrites files that already exist", async () => {
      root = new FakeDir("repo", {
        hittable: { "env.json": '{"BASE_URL":"https://api.example.com"}', notes: { "mine.md": "keep" }, testcollection: { "test.hit": "custom" } },
      });
      const ctx = await openedWorkspace();
      // The only addition is the .gitignore that keeps env.local.json out of git (FR-17).
      expect(root.toTree()).toEqual({
        hittable: {
          ".gitignore": "env.local.json\n",
          "env.json": '{"BASE_URL":"https://api.example.com"}',
          notes: { "mine.md": "keep" },
          testcollection: { "test.hit": "custom" },
        },
      });
      expect(ctx().envContent).toEqual({ BASE_URL: "https://api.example.com" });
    });

    it("reads the whole folder into the explorer tree, with paths", async () => {
      const ctx = await openedWorkspace();
      expect(ctx().tree.map((n) => n.name).sort()).toEqual(["README.md", "hittable", "todo.txt"]);
      const hittable = ctx().tree.find((n) => n.name === "hittable")!;
      expect(hittable.kind).toBe("directory");
      expect(hittable.path).toEqual(["hittable"]);
      expect(hittable.children!.map((n) => n.name).sort()).toEqual([".gitignore", "env.json", "notes", "testcollection"]);
      expect(hittable.children!.find((n) => n.name === "notes")!.children![0].path).toEqual(["hittable", "notes", "sample.md"]);
    });

    // FR-17: env.local.json holds values that must not reach git.
    describe("env.local.json", () => {
      it("overrides env.json values", async () => {
        root = new FakeDir("repo", {
          hittable: { "env.json": '{"BASE_URL":"https://api.example.com","TOKEN":"placeholder"}', "env.local.json": '{"TOKEN":"real-secret"}' },
        });
        const ctx = await openedWorkspace();
        expect(ctx().envContent).toEqual({ BASE_URL: "https://api.example.com", TOKEN: "real-secret" });
      });

      it("is added to an existing .gitignore once, keeping what is there", async () => {
        root = new FakeDir("repo", { hittable: { "env.json": "{}", ".gitignore": "*.log" } });
        await openedWorkspace();
        expect((root.toTree().hittable as Record<string, string>)[".gitignore"]).toBe("*.log\nenv.local.json\n");
        await openedWorkspace(); // opening again does not add it twice
        expect((root.toTree().hittable as Record<string, string>)[".gitignore"]).toBe("*.log\nenv.local.json\n");
      });

      it("a broken env.local.json is ignored rather than breaking the folder", async () => {
        root = new FakeDir("repo", { hittable: { "env.json": '{"A":"1"}', "env.local.json": "{oops" } });
        const ctx = await openedWorkspace();
        expect(ctx().envContent).toEqual({ A: "1" });
      });
    });

    it("remembers the folder handle", async () => {
      await openedWorkspace();
      expect(handleStore.handle).toBe(root);
    });

    it("a cancelled picker opens nothing and logs no error", async () => {
      const err = vi.spyOn(console, "error").mockImplementation(() => {});
      window.showDirectoryPicker = vi.fn(async () => { throw new DOMException("cancel", "AbortError"); }) as unknown as typeof window.showDirectoryPicker;
      const ctx = mount();
      let ok: boolean | undefined;
      await act(async () => { ok = await ctx().openFolder(); });
      expect(ok).toBe(false);
      expect(ctx().backendKind).toBeNull();
      expect(ctx().connectingStage).toBeNull();
      expect(err).not.toHaveBeenCalled();
    });

    it("a second click while the picker is open is ignored, not an error", async () => {
      let resolve!: (h: FileSystemDirectoryHandle) => void;
      const picker = vi.fn(() => new Promise<FileSystemDirectoryHandle>((r) => { resolve = r; }));
      window.showDirectoryPicker = picker as unknown as typeof window.showDirectoryPicker;
      const ctx = mount();
      let first!: Promise<boolean>;
      let second: boolean | undefined;
      await act(async () => {
        first = ctx().openFolder();
        second = await ctx().openFolder();
      });
      expect(second).toBe(false);
      expect(picker).toHaveBeenCalledTimes(1);
      await act(async () => { resolve(asDir(root)); await first; });
      expect(ctx().backendKind).toBe("folder");
    });

    it("a refused picker shows a message on the page instead of a console error", async () => {
      const err = vi.spyOn(console, "error").mockImplementation(() => {});
      const notify = vi.fn();
      window.showDirectoryPicker = vi.fn(async () => { throw new DOMException("blocked", "NotAllowedError"); }) as unknown as typeof window.showDirectoryPicker;
      const ctxRef: { current: Ctx | null } = { current: null };
      function Probe() {
        const c = useWorkspace();
        useEffect(() => { ctxRef.current = c; });
        return null;
      }
      render(<NotificationContext.Provider value={{ notify }}><WorkspaceProvider><Probe /></WorkspaceProvider></NotificationContext.Provider>);
      await act(async () => { await ctxRef.current!.openFolder(); });
      expect(notify).toHaveBeenCalledWith(expect.objectContaining({ mode: "error", title: "Couldn't open the folder" }));
      expect(err).not.toHaveBeenCalled();
    });
  });

  it("closing the workspace forgets the folder", async () => {
    const ctx = await openedWorkspace();
    act(() => ctx().closeWorkspace());
    expect(ctx().backendKind).toBeNull();
    expect(ctx().tree).toEqual([]);
    expect(ctx().label).toBe("");
    await waitFor(() => expect(handleStore.handle).toBeNull());
  });

  describe("reconnecting", () => {
    it("reopens the saved folder when the browser kept permission", async () => {
      root = new FakeDir("repo", { hittable: { "env.json": '{"K":"v"}' } });
      handleStore.handle = root;
      const ctx = mount();
      await act(async () => { expect(await ctx().reconnectFolder()).toBe(true); });
      expect(ctx().backendKind).toBe("folder");
      expect(ctx().label).toBe("repo");
      expect(ctx().envContent).toEqual({ K: "v" });
    });

    it("without a prompt, keeps the handle when permission must be asked for", async () => {
      root.permission = "prompt";
      handleStore.handle = root;
      const ctx = mount();
      await act(async () => { expect(await ctx().reconnectFolder()).toBe(false); });
      expect(ctx().backendKind).toBeNull();
      expect(handleStore.handle).toBe(root);
    });

    it("drops the handle when permission is refused on asking", async () => {
      root.permission = "denied";
      handleStore.handle = root;
      const ctx = mount();
      await act(async () => { expect(await ctx().reconnectFolder(true)).toBe(false); });
      expect(handleStore.handle).toBeNull();
    });

    it("does nothing without a saved folder", async () => {
      const ctx = mount();
      await act(async () => { expect(await ctx().reconnectFolder(true)).toBe(false); });
      expect(ctx().backendKind).toBeNull();
    });
  });

  describe("files", () => {
    async function openFile(ctx: () => Ctx, path: string) {
      act(() => ctx().setActiveFile(path.split("/")));
      await waitFor(() => expect(ctx().isFileLoaded).toBe(true));
      return root.at(path) as FakeFile;
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

    it("an image is shown, not loaded as text", async () => {
      const png = new FakeFile("logo.png");
      png.bytes = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x00]);
      root.children.set("logo.png", png);
      URL.createObjectURL = vi.fn(() => "blob:img"); // not in jsdom
      URL.revokeObjectURL = vi.fn();
      const ctx = await openedWorkspace();
      await openFile(ctx, "logo.png");
      expect(ctx().fileView).toEqual({ kind: "image", label: "PNG image" });
      expect(ctx().fileSize).toBe(5);
      expect(ctx().fileUrl).toBe("blob:img");
      expect(ctx().rawTextContent).toBe("");
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

    describe('openTool("env")', () => {
      it("opens hittable/env.json", async () => {
        const ctx = await openedWorkspace();
        act(() => { expect(openTool("env")).toBe(true); });
        await waitFor(() => expect(ctx().activeFile?.path).toEqual(["hittable", "env.json"]));
      });

      it("creates env.json when it is missing", async () => {
        const ctx = await openedWorkspace();
        await act(async () => { await (root.at("hittable") as FakeDir).removeEntry("env.json"); await ctx().refreshTree(); });
        act(() => { openTool("env"); });
        await waitFor(() => expect(ctx().activeFile?.path).toEqual(["hittable", "env.json"]));
        expect((root.at("hittable/env.json") as FakeFile).content).toBe("{}\n");
      });

      it("is not claimed when no workspace is open", () => {
        mount();
        let claimed = true;
        act(() => { claimed = openTool("env"); });
        expect(claimed).toBe(false);
      });
    });
  });

  describe("explorer operations", () => {
    it("creating a .hit file writes a blank request template", async () => {
      const ctx = await openedWorkspace();
      await act(() => ctx().createEntry(["hittable", "users.hit"], "file"));
      expect(JSON.parse((root.at("hittable/users.hit") as FakeFile).content)).toEqual({
        method: "GET", url: "", headers: {}, params: {}, body: "", response: null,
      });
      expect(ctx().tree.find((n) => n.name === "hittable")!.children!.some((c) => c.name === "users.hit")).toBe(true);
    });

    it("creating any other file makes it empty (or holds the given text); creating a folder makes a folder", async () => {
      const ctx = await openedWorkspace();
      await act(() => ctx().createEntry(["notes.txt"], "file"));
      await act(() => ctx().createEntry(["api"], "directory"));
      await act(() => ctx().createEntry(["hittable", "shared", "x.hit"], "file", "given"));
      expect((root.at("notes.txt") as FakeFile).content).toBe("");
      expect(root.at("api")!.kind).toBe("directory");
      expect((root.at("hittable/shared/x.hit") as FakeFile).content).toBe("given");
    });

    it("creating a file with a name that already exists fails and keeps the existing file", async () => {
      const ctx = await openedWorkspace();
      await expect(act(() => ctx().createEntry(["README.md"], "file"))).rejects.toThrow(/already exists/);
      expect((root.at("README.md") as FakeFile).content).toBe("# repo");
    });

    it("reserved names in hittable/ cannot be created, renamed or deleted", async () => {
      const ctx = await openedWorkspace();
      await expect(act(() => ctx().deleteEntry(["hittable", "env.json"]))).rejects.toThrow(/reserved/);
      await expect(act(() => ctx().renameEntry(["hittable", "notes"], "old"))).rejects.toThrow(/reserved/);
      await expect(act(() => ctx().renameEntry(["todo.txt"], "env.json"))).rejects.toThrow(/reserved/);
      expect(root.at("hittable/env.json")).toBeDefined();
      expect(root.at("hittable/notes")).toBeDefined();
    });

    it("deleting a folder removes it and everything inside, closing an open file in it", async () => {
      const ctx = await openedWorkspace();
      act(() => ctx().setActiveFile(["hittable", "testcollection", "test.hit"]));
      await act(() => ctx().deleteEntry(["hittable", "testcollection"]));
      expect(root.at("hittable/testcollection")).toBeUndefined();
      expect(ctx().tree.find((n) => n.name === "hittable")!.children!.map((c) => c.name)).not.toContain("testcollection");
      expect(ctx().activeFile).toBeNull();
    });

    it("renaming a file keeps its content", async () => {
      const ctx = await openedWorkspace();
      await act(() => ctx().renameEntry(["README.md"], "INTRO.md"));
      expect(root.at("README.md")).toBeUndefined();
      expect((root.at("INTRO.md") as FakeFile).content).toBe("# repo");
    });

    it("renaming a folder copies its whole subtree", async () => {
      const ctx = await openedWorkspace();
      await act(() => ctx().renameEntry(["hittable", "testcollection"], "users"));
      expect(root.at("hittable/testcollection")).toBeUndefined();
      expect(JSON.parse((root.at("hittable/users/test.hit") as FakeFile).content).method).toBe("GET");
    });

    it("renaming onto an existing name fails and loses nothing", async () => {
      root = new FakeDir("repo", { "a.hit": "A", "b.hit": "B" });
      const ctx = await openedWorkspace();
      await expect(act(() => ctx().renameEntry(["a.hit"], "b.hit"))).rejects.toThrow(/already exists/);
      expect((root.at("a.hit") as FakeFile).content).toBe("A");
      expect((root.at("b.hit") as FakeFile).content).toBe("B");
    });
  });

  describe("any backend", () => {
    function fakeRepo(over: Partial<WorkspaceBackend> = {}): WorkspaceBackend {
      root = new FakeDir("repo", { hittable: { "env.json": '{"A":"1"}', "a.hit": "{}" } });
      return { ...createFolderBackend(asDir(root)), kind: "github", label: "me/repo", ...over };
    }

    it("shows the backend's label, status and read-only flag", async () => {
      const ctx = mount();
      await act(() => ctx().openBackend(fakeRepo({ readOnly: true, status: () => ({ "hittable/a.hit": "M" }) })));
      expect(ctx().backendKind).toBe("github");
      expect(ctx().label).toBe("me/repo");
      expect(ctx().readOnly).toBe(true);
      expect(ctx().status).toEqual({ "hittable/a.hit": "M" });
      expect(ctx().envContent).toEqual({ A: "1" });
    });

    it("a read-only workspace never writes", async () => {
      const ctx = mount();
      await act(() => ctx().openBackend(fakeRepo({ readOnly: true })));
      act(() => ctx().setActiveFile(["hittable", "a.hit"]));
      await waitFor(() => expect(ctx().isFileLoaded).toBe(true));
      await act(async () => { await ctx().saveRawTextContent("changed"); });
      expect((root.at("hittable/a.hit") as FakeFile).content).toBe("{}");
      await expect(act(() => ctx().createEntry(["new.hit"], "file"))).rejects.toThrow(/read-only/);
      // openTool("env") does not create a missing env.json either
      await act(async () => { await (root.at("hittable") as FakeDir).removeEntry("env.json"); await ctx().refreshTree(); });
      act(() => { openTool("env"); });
      await act(async () => {});
      expect(root.at("hittable/env.json")).toBeUndefined();
    });

    it("re-reads status and the tree when the backend reports a change", async () => {
      let notify = () => {};
      let status = {};
      const ctx = mount();
      await act(() => ctx().openBackend(fakeRepo({ status: () => status, subscribe: (fn) => { notify = fn; return () => {}; } })));
      status = { "hittable/new.hit": "A" };
      (root.at("hittable") as FakeDir).children.set("new.hit", new FakeFile("new.hit", "{}"));
      await act(async () => { notify(); });
      await waitFor(() => expect(ctx().status).toEqual({ "hittable/new.hit": "A" }));
      expect(ctx().tree[0].children!.map((c) => c.name)).toContain("new.hit");
    });

    it("unsubscribes when the workspace closes", async () => {
      const unsubscribe = vi.fn();
      const ctx = mount();
      await act(() => ctx().openBackend(fakeRepo({ subscribe: () => unsubscribe })));
      act(() => ctx().closeWorkspace());
      expect(unsubscribe).toHaveBeenCalled();
    });
  });
});
