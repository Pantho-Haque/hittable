// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import SharePreview from "@/components/workspace/SharePreview";
import { NotificationProvider } from "@/context/notifyContext";
import { encodeShare } from "@/utils/shareLink";
import type { THittableCurlJson } from "@/types";
import type { WsNode } from "@/services/workspace/backend";

const ws = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
vi.mock("@/context/workspaceContext", () => ({ useWorkspace: () => ws.current }));
const push = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

const mountData = (ui: React.ReactNode) => render(<NotificationProvider>{ui}</NotificationProvider>);

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

const taken: WsNode = {
  name: "hittable", path: ["hittable"], kind: "directory", children: [
    { name: "shared", path: ["hittable", "shared"], kind: "directory", children: [
      { name: "POST-api.io-users.hit", path: ["hittable", "shared", "POST-api.io-users.hit"], kind: "file" },
    ] },
  ],
};

beforeEach(() => {
  sessionStorage.clear();
  push.mockReset();
  window.history.replaceState(null, "", "/hittable");
  ws.current = { backendKind: "folder", readOnly: false, tree: [], createEntry: vi.fn(async () => {}), setActiveFile: vi.fn() };
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

  it("Import writes hittable/shared/<name>.hit through the workspace and opens it", async () => {
    openLink();
    mountData(<SharePreview />);
    const user = userEvent.setup();
    await user.click(within(await preview()).getByRole("button", { name: "Import" }));
    const { createEntry, setActiveFile } = ws.current as { createEntry: ReturnType<typeof vi.fn>; setActiveFile: ReturnType<typeof vi.fn> };
    await waitFor(() => expect(setActiveFile).toHaveBeenCalledWith(["hittable", "shared", "POST-api.io-users.hit"]));
    const [path, kind, text] = createEntry.mock.calls[0];
    expect(path).toEqual(["hittable", "shared", "POST-api.io-users.hit"]);
    expect(kind).toBe("file");
    expect(JSON.parse(text)).toEqual({
      method: "POST",
      url: shared.url,
      headers: { Authorization: "Bearer <<TOKEN>>", "Content-Type": "application/json" },
      params: { x: "1" },
      body: shared.body,
      response: null,
    });
    expect(window.location.hash).toBe("");
    expect(await screen.findByText("Imported into hittable/shared")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Shared request" })).not.toBeInTheDocument());
  });

  it("Import uses the shared name and keeps file names unique", async () => {
    ws.current.tree = [taken];
    openLink();
    mountData(<SharePreview />);
    await userEvent.setup().click(within(await preview()).getByRole("button", { name: "Import" }));
    const { createEntry } = ws.current as { createEntry: ReturnType<typeof vi.fn> };
    await waitFor(() => expect(createEntry).toHaveBeenCalled());
    expect(createEntry.mock.calls[0][0]).toEqual(["hittable", "shared", "POST-api.io-users-2.hit"]);
  });

  it("a failed import says so and keeps the preview open", async () => {
    ws.current.createEntry = vi.fn(async () => { throw new Error("disk full"); });
    openLink(shared, "List users");
    mountData(<SharePreview />);
    await userEvent.setup().click(within(await preview()).getByRole("button", { name: "Import" }));
    expect(await screen.findByText("disk full")).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "Shared request" })).toBeInTheDocument();
    expect((ws.current.createEntry as ReturnType<typeof vi.fn>).mock.calls[0][0]).toEqual(["hittable", "shared", "List-users.hit"]);
  });

  it("with no workspace open, it explains instead of offering Import", async () => {
    ws.current = { backendKind: null, readOnly: false, tree: [] };
    openLink();
    mountData(<SharePreview />);
    const d = await preview();
    expect(within(d).getByText(/open a local folder or a GitHub repository — this preview comes back there/)).toBeInTheDocument();
    expect(within(d).queryByRole("button", { name: "Import" })).not.toBeInTheDocument();
    await userEvent.setup().click(within(d).getByRole("button", { name: "Dismiss" }));
    expect(window.location.hash).toBe("");
    expect(sessionStorage.getItem("hittable_pending_share")).toBeNull(); // dismissed means forgotten
  });

  // A link lands on the launcher (nothing open). It is kept for this tab and
  // the preview comes back once a writable workspace is open.
  it("a link opened with no workspace comes back when one opens", async () => {
    ws.current = { backendKind: null, readOnly: false, tree: [] };
    openLink();
    const link = window.location.hash;
    const view = mountData(<SharePreview />);
    await preview();
    expect(sessionStorage.getItem("hittable_pending_share")).toBe(link);

    // The user navigates away (hash gone), then opens a folder.
    window.history.replaceState(null, "", window.location.pathname);
    window.dispatchEvent(new HashChangeEvent("hashchange"));
    ws.current = { ...ws.current, backendKind: "folder", createEntry: vi.fn(), setActiveFile: vi.fn() };
    view.rerender(<NotificationProvider><SharePreview /></NotificationProvider>);
    const d = await preview();
    expect(within(d).getByRole("button", { name: "Import" })).toBeInTheDocument();
    expect(sessionStorage.getItem("hittable_pending_share")).toBeNull();
  });

  it("offers to open a local folder right from the preview, keeping the link", async () => {
    Object.defineProperty(window, "showDirectoryPicker", { value: vi.fn(), configurable: true });
    const openFolder = vi.fn(async () => true);
    ws.current = { backendKind: null, readOnly: false, tree: [], openFolder };
    openLink();
    mountData(<SharePreview />);
    const d = await preview();
    await userEvent.setup().click(within(d).getByRole("button", { name: "Open a local folder" }));
    expect(openFolder).toHaveBeenCalled();
    expect(push).toHaveBeenCalledWith(expect.stringMatching(/^\/hittable\/folder#share=/));
    delete (window as unknown as { showDirectoryPicker?: unknown }).showDirectoryPicker;
  });

  it("a read-only workspace explains it can't import", async () => {
    ws.current = { backendKind: "github", readOnly: true, tree: [] };
    openLink();
    mountData(<SharePreview />);
    const d = await preview();
    expect(within(d).getByText(/read-only/)).toBeInTheDocument();
    expect(within(d).queryByRole("button", { name: "Import" })).not.toBeInTheDocument();
  });

  it("Dismiss closes the preview and clears the hash", async () => {
    openLink();
    mountData(<SharePreview />);
    const user = userEvent.setup();
    await user.click(within(await preview()).getByRole("button", { name: "Dismiss" }));
    expect(window.location.hash).toBe("");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(ws.current.createEntry).not.toHaveBeenCalled();
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

});
