// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import FolderWorkspace from "@/app/hittable/folder/page";
import { WorkspaceProvider } from "@/context/workspaceContext";
import { NotificationProvider } from "@/context/notifyContext";
import { ShortcutProvider } from "@/context/ShortcutKeypressProvider";
import { FakeDir } from "../helpers/fakeFs";

const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

const handleStore = vi.hoisted(() => ({ handle: null as unknown }));
vi.mock("@/services/workspace/HandleStore", () => ({
  saveDirectoryHandle: vi.fn(async (h: unknown) => { handleStore.handle = h; }),
  loadDirectoryHandle: vi.fn(async () => handleStore.handle),
  removeDirectoryHandle: vi.fn(async () => { handleStore.handle = null; }),
  verifyPermission: vi.fn(async (h: FakeDir, prompt = true) =>
    (await h.queryPermission()) === "granted" || (prompt && (await h.requestPermission()) === "granted")),
}));

function mount() {
  render(
    <NotificationProvider>
      <ShortcutProvider>
        <WorkspaceProvider>
          <FolderWorkspace />
        </WorkspaceProvider>
      </ShortcutProvider>
    </NotificationProvider>,
  );
  return userEvent.setup();
}

beforeEach(() => {
  router.push.mockReset();
  router.replace.mockReset();
  Element.prototype.scrollIntoView = () => {};
  handleStore.handle = new FakeDir("my-api", { "README.md": "# hi" });
});

describe("/hittable/folder", () => {
  it("reopens the last folder when the browser kept permission", async () => {
    mount();
    expect(await screen.findByRole("treeitem", { name: "README.md" })).toBeInTheDocument();
    expect(screen.getAllByText("my-api").length).toBeGreaterThan(0);
    expect(screen.getByText(/Select a file in the explorer/)).toBeInTheDocument();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("goes back to the launcher when permission must be asked for", async () => {
    (handleStore.handle as FakeDir).permission = "prompt";
    mount();
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/hittable"));
  });

  it("goes back to the launcher with no saved folder", async () => {
    handleStore.handle = null;
    mount();
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/hittable"));
  });

  it("Close forgets the folder and returns to the launcher", async () => {
    const user = mount();
    await screen.findByRole("treeitem", { name: "README.md" });
    await user.click(screen.getByRole("button", { name: "Close" }));
    expect(router.push).toHaveBeenCalledWith("/hittable");
    await waitFor(() => expect(handleStore.handle).toBeNull());
    expect(screen.queryByRole("treeitem", { name: "README.md" })).not.toBeInTheDocument();
  });
});
