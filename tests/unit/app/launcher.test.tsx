// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import Launcher from "@/app/hittable/page";
import { WorkspaceProvider } from "@/context/workspaceContext";
import { FakeDir, asDir } from "../helpers/fakeFs";

const push = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

const handleStore = vi.hoisted(() => ({ handle: null as unknown }));
vi.mock("@/services/workspace/HandleStore", () => ({
  saveDirectoryHandle: vi.fn(async (h: unknown) => { handleStore.handle = h; }),
  loadDirectoryHandle: vi.fn(async () => handleStore.handle),
  removeDirectoryHandle: vi.fn(async () => { handleStore.handle = null; }),
  verifyPermission: vi.fn(async (h: FakeDir) => (await h.queryPermission()) === "granted"),
}));

let root: FakeDir;

function mount(url = "/hittable") {
  window.history.replaceState(null, "", url);
  render(<WorkspaceProvider><Launcher /></WorkspaceProvider>);
  return userEvent.setup();
}

beforeEach(() => {
  localStorage.clear();
  push.mockReset();
  handleStore.handle = null;
  root = new FakeDir("my-api", { "README.md": "# hi" });
  window.showDirectoryPicker = vi.fn(async () => asDir(root)) as unknown as typeof window.showDirectoryPicker;
});
afterEach(() => {
  delete (window as Partial<Window>).showDirectoryPicker;
});

describe("Workspaces launcher", () => {
  it("offers a local folder and GitHub", () => {
    mount();
    expect(screen.getByRole("heading", { level: 1, name: "Open a workspace" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open a local folder" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Sign in with GitHub" })).toHaveAttribute("href", "/api/github/login?next=/hittable");
  });

  it("Open a local folder scaffolds it and goes to the folder workspace", async () => {
    const user = mount();
    await user.click(screen.getByRole("button", { name: "Open a local folder" }));
    await waitFor(() => expect(push).toHaveBeenCalledWith("/hittable/folder"));
    expect(root.at("hittable/env.json")).toBeDefined();
  });

  it("a cancelled picker stays here", async () => {
    window.showDirectoryPicker = vi.fn(async () => { throw new DOMException("cancel", "AbortError"); }) as unknown as typeof window.showDirectoryPicker;
    const user = mount();
    await user.click(screen.getByRole("button", { name: "Open a local folder" }));
    expect(push).not.toHaveBeenCalled();
  });

  it("explains when the browser can't open folders, and points to GitHub", () => {
    delete (window as Partial<Window>).showDirectoryPicker;
    mount();
    expect(screen.queryByRole("button", { name: "Open a local folder" })).not.toBeInTheDocument();
    expect(screen.getByRole("note")).toHaveTextContent(/Chrome or Edge.*GitHub repository instead/);
  });

  it("reconnects straight away when the browser kept permission", async () => {
    handleStore.handle = root;
    const user = mount();
    await user.click(await screen.findByRole("button", { name: "Reconnect my-api" }));
    await waitFor(() => expect(push).toHaveBeenCalledWith("/hittable/folder"));
    expect(window.showDirectoryPicker).not.toHaveBeenCalled();
  });

  it("otherwise reconnects through the picker, opened on the saved folder", async () => {
    root.permission = "prompt";
    handleStore.handle = root;
    const user = mount();
    await user.click(await screen.findByRole("button", { name: "Reconnect my-api" }));
    await waitFor(() => expect(push).toHaveBeenCalledWith("/hittable/folder"));
    expect(window.showDirectoryPicker).toHaveBeenCalledWith({ mode: "readwrite", startIn: root });
  });

  describe("sign-in errors", () => {
    it.each([
      ["state_mismatch", /couldn't be verified/],
      ["missing_verifier", /expired/],
      ["exchange_failed", /didn't complete the sign-in/],
      ["denied", /cancelled on GitHub/],
      ["config", /isn't set up on this server/],
      ["something_else", /didn't work/],
    ])("?auth_error=%s gets a friendly message", (code, text) => {
      mount(`/hittable?auth_error=${code}`);
      expect(screen.getByRole("alert")).toHaveTextContent(text);
    });

    it("no error, no alert", () => {
      mount();
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });
  });

  describe("old Local-mode collections (FR-G10)", () => {
    const collections = [
      {
        collectionName: "Users API",
        items: [
          { type: "route", name: "List", curl: "curl -X GET '<<BASE>>/users' -H 'Authorization: Bearer <<TOKEN>>'", response: "" },
          { type: "folder", name: "Admin", items: [{ type: "route", name: "Create", curl: "curl -X POST '<<BASE>>/users' -d '{\"a\":1}'", response: "" }] },
        ],
        env: { BASE: "https://api.example.com" },
        secrets: { TOKEN: "s3cret" },
      },
      { collectionName: "Legacy", curls: [{ name: "Ping", curl: "curl https://x.dev/ping", response: "" }], env: {} },
    ];
    const banner = () => screen.findByRole("region", { name: /old browser collections are still here/ });

    let blobs: Blob[];
    beforeEach(() => {
      blobs = [];
      URL.createObjectURL = vi.fn((b: Blob) => { blobs.push(b); return `blob:${blobs.length}`; }); // not in jsdom
      URL.revokeObjectURL = vi.fn();
      vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    });
    afterEach(() => vi.restoreAllMocks());

    it("no old data, no banner", async () => {
      mount();
      await screen.findByRole("button", { name: "Open a local folder" });
      expect(screen.queryByRole("region", { name: /old browser collections/ })).not.toBeInTheDocument();
    });

    it("Download saves one Postman v2.1 collection per collection, without secret values", async () => {
      localStorage.setItem("hittable", JSON.stringify(collections));
      const user = mount();
      await user.click(within(await banner()).getByRole("button", { name: "Download" }));
      expect(blobs).toHaveLength(2);
      const users = JSON.parse(await blobs[0].text());
      expect(users.info).toEqual({ name: "Users API", schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" });
      expect(users.variable).toEqual([
        { key: "BASE", value: "https://api.example.com", type: "string" },
        { key: "TOKEN", value: "", type: "string" },
      ]);
      expect(JSON.stringify(users)).not.toContain("s3cret");
      expect(users.item[0]).toEqual({
        name: "List",
        request: { method: "GET", header: [{ key: "Authorization", value: "Bearer {{TOKEN}}" }], url: "{{BASE}}/users" },
      });
      expect(users.item[1].name).toBe("Admin");
      const create = users.item[1].item[0].request;
      expect(create).toMatchObject({ method: "POST", url: "{{BASE}}/users", body: { mode: "raw" } });
      expect(JSON.parse(create.body.raw)).toEqual({ a: 1 });
      const legacy = JSON.parse(await blobs[1].text());
      expect(legacy.item[0]).toMatchObject({ name: "Ping", request: { method: "GET", url: "https://x.dev/ping" } });
      // the data is kept until dismissed
      expect(localStorage.getItem("hittable")).not.toBeNull();
    });

    it("Dismiss hides it for good but keeps the data", async () => {
      localStorage.setItem("hittable", JSON.stringify(collections));
      const user = mount();
      await user.click(within(await banner()).getByRole("button", { name: "Dismiss" }));
      expect(screen.queryByRole("region", { name: /old browser collections/ })).not.toBeInTheDocument();
      expect(localStorage.getItem("hittable_legacy_dismissed")).toBe("1");
      expect(localStorage.getItem("hittable")).not.toBeNull();
    });

    it("stays hidden once dismissed, or when the stored data is unreadable", async () => {
      localStorage.setItem("hittable", JSON.stringify(collections));
      localStorage.setItem("hittable_legacy_dismissed", "1");
      mount();
      await screen.findByRole("button", { name: "Open a local folder" });
      expect(screen.queryByRole("region", { name: /old browser collections/ })).not.toBeInTheDocument();
    });

    it("blocked storage shows no banner and breaks nothing", async () => {
      vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("blocked"); });
      mount();
      await screen.findByRole("button", { name: "Open a local folder" });
      expect(screen.queryByRole("region", { name: /old browser collections/ })).not.toBeInTheDocument();
    });
  });
});
