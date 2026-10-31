// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SharePreview from "@/components/hittable/SharePreview";
import { mountData } from "../../helpers/dataHarness";
import { encodeShare } from "@/utils/shareLink";
import { curlConverter } from "@/utils/curlConverter";
import { findRoute } from "@/utils/treeHelpers";
import type { THittableCurlJson } from "@/types";

const push = vi.hoisted(() => ({ fn: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: (u: string) => push.fn(u) }) }));

const ws = vi.hoisted(() => ({ current: { mode: "local" } as Record<string, unknown> }));
vi.mock("@/context/workspaceContext", () => ({ useWorkspace: () => ws.current }));

const shared: THittableCurlJson = {
  method: "POST",
  url: "https://api.io/users?x=1",
  headers: JSON.stringify({ Authorization: "Bearer abc", "Content-Type": "application/json" }),
  params: JSON.stringify({ x: "1" }),
  body: '<img src=x onerror="alert(1)">',
};

function openLink(req = shared, name?: string) {
  const res = encodeShare(req, name);
  if (!res.ok) throw new Error(res.error);
  window.history.replaceState(null, "", `/hittable#${res.fragment}`);
}

const preview = () => screen.findByRole("dialog", { name: "Shared request" });

beforeEach(() => {
  window.history.replaceState(null, "", "/hittable");
  ws.current = { mode: "local" };
  push.fn = vi.fn();
});

describe("SharePreview", () => {
  it("renders nothing without a share fragment", () => {
    mountData(<SharePreview />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("shows the decoded request as text, with the placeholder and the nothing-sent note", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    const postSpy = vi.spyOn(window, "postMessage");
    openLink();
    mountData(<SharePreview />);
    const d = await preview();
    expect(within(d).getByText("POST")).toBeInTheDocument();
    expect(within(d).getByText("https://api.io/users?x=1")).toBeInTheDocument();
    expect(within(d).getByText("Bearer <<TOKEN>>")).toBeInTheDocument();
    expect(within(d).getByText("application/json")).toBeInTheDocument();
    // The body is text, never markup.
    expect(within(d).getByText(shared.body)).toBeInTheDocument();
    expect(d.querySelector("img")).toBeNull();
    expect(within(d).getByText(/Nothing has been sent/)).toBeInTheDocument();
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(postSpy.mock.calls.filter(([m]) => (m as { type?: string })?.type === "HITTABLE_REQUEST")).toEqual([]);
  });

  it("opens when the hash changes to a share link", async () => {
    mountData(<SharePreview />);
    act(() => {
      openLink();
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(await preview()).toBeInTheDocument();
  });

  it("Import adds the request to a new Shared collection and selects it", async () => {
    openLink();
    const { ctx } = mountData(<SharePreview />);
    const user = userEvent.setup();
    await user.click(within(await preview()).getByRole("button", { name: "Import" }));
    await waitFor(() => expect(ctx().collections.find((c) => c.collectionName === "Shared")).toBeTruthy());
    const col = ctx().collections.find((c) => c.collectionName === "Shared")!;
    expect(col.env).toEqual({});
    expect(col.secrets).toEqual({});
    const route = findRoute(col.items, [], "POST api.io/users");
    expect(route).toBeTruthy();
    const req = curlConverter(route!.curl);
    expect(req.url).toBe(shared.url);
    expect(JSON.parse(req.headers)).toMatchObject({ Authorization: "Bearer <<TOKEN>>" });
    expect(push.fn).toHaveBeenCalledWith("/hittable?c=Shared&r=POST%20api.io%2Fusers");
    expect(window.location.hash).toBe("");
    expect(await screen.findByText("Imported into Shared")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Shared request" })).not.toBeInTheDocument());
  });

  it("Import uses the shared name and keeps names unique in an existing Shared collection", async () => {
    openLink(shared, "List");
    const { ctx } = mountData(<SharePreview />, {
      collections: [{ collectionName: "Shared", items: [{ type: "route", name: "List", curl: "curl x", response: "" }], env: { A: "1" }, secrets: {} }],
      collection: "Shared",
      route: "List",
    });
    const user = userEvent.setup();
    await user.click(within(await preview()).getByRole("button", { name: "Import" }));
    await waitFor(() => expect(ctx().collections[0].items.map((i) => i.name)).toEqual(["List", "List (2)"]));
    expect(ctx().collections[0].env).toEqual({ A: "1" });
  });

  it("Dismiss closes the preview and clears the hash", async () => {
    openLink();
    const { ctx } = mountData(<SharePreview />);
    const before = ctx().collections;
    const user = userEvent.setup();
    await user.click(within(await preview()).getByRole("button", { name: "Dismiss" }));
    expect(window.location.hash).toBe("");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(ctx().collections).toEqual(before);
  });

  it("a damaged link shows an error dialog, and Dismiss clears it", async () => {
    window.history.replaceState(null, "", "/hittable#share=not-really-a-share");
    mountData(<SharePreview />);
    const d = await screen.findByRole("dialog", { name: "Can't open share link" });
    expect(within(d).getByText(/damaged or too large/)).toBeInTheDocument();
    await userEvent.setup().click(within(d).getByRole("button", { name: "Dismiss" }));
    expect(window.location.hash).toBe("");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("folder mode writes hittable/shared/<name>.hit and opens it", async () => {
    const files = new Map<string, string>();
    const fileHandle = (name: string) => ({
      kind: "file",
      name,
      createWritable: async () => ({ write: async (c: string) => void files.set(name, c), close: async () => {} }),
    });
    const sharedDir = {
      getFileHandle: async (name: string, opts?: { create?: boolean }) => {
        if (!files.has(name) && !opts?.create) throw new DOMException("missing", "NotFoundError");
        return fileHandle(name);
      },
    };
    files.set("POST-api.io-users.hit", "{}");
    const hittableDir = { name: "hittable", getDirectoryHandle: vi.fn(async () => sharedDir) };
    const setActiveFile = vi.fn();
    const refreshTree = vi.fn(async () => {});
    ws.current = { mode: "directory", hittableDir, directoryHandle: {}, refreshTree, setActiveFile };

    openLink();
    mountData(<SharePreview />);
    await userEvent.setup().click(within(await preview()).getByRole("button", { name: "Import" }));
    await waitFor(() => expect(setActiveFile).toHaveBeenCalled());
    expect(hittableDir.getDirectoryHandle).toHaveBeenCalledWith("shared", { create: true });
    const name = "POST-api.io-users-2.hit"; // the first name was taken
    expect(setActiveFile.mock.calls[0][0].path).toEqual(["hittable", "shared", name]);
    expect(JSON.parse(files.get(name)!)).toEqual({
      method: "POST",
      url: shared.url,
      headers: { Authorization: "Bearer <<TOKEN>>", "Content-Type": "application/json" },
      params: { x: "1" },
      body: shared.body,
      response: null,
    });
    expect(refreshTree).toHaveBeenCalled();
    expect(window.location.hash).toBe("");
  });
});
