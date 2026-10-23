// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FakeDir, asDir } from "../../helpers/fakeFs";

vi.mock("@/services/workspace/HandleStore", () => ({
  saveDirectoryHandle: vi.fn(async () => {}),
  loadDirectoryHandle: vi.fn(async () => null),
  removeDirectoryHandle: vi.fn(async () => {}),
  verifyPermission: vi.fn(async () => true),
}));

// Support is detected once at import time, so each test imports fresh.
async function mount() {
  vi.resetModules();
  const { WorkspaceProvider } = await import("@/context/workspaceContext");
  const { default: WorkspaceSwitcher } = await import("@/components/workspace/WorkspaceSwitcher");
  render(<WorkspaceProvider><WorkspaceSwitcher /></WorkspaceProvider>);
  return userEvent.setup();
}

beforeEach(() => localStorage.clear());
afterEach(() => { delete (window as Partial<Window>).showDirectoryPicker; });

describe("Local / Directory switch", () => {
  it("in a Chromium browser it opens a folder, then switches back to Local", async () => {
    const root = new FakeDir("repo");
    window.showDirectoryPicker = vi.fn(async () => asDir(root)) as unknown as typeof window.showDirectoryPicker;
    const user = await mount();
    const toLocal = () => screen.getByRole("button", { name: "Switch to local workspace" });

    expect(screen.getByRole("button", { name: "Open a directory workspace" })).toHaveTextContent("Local");
    await user.click(screen.getByRole("button", { name: "Open a directory workspace" }));
    expect(await screen.findByRole("button", { name: "Switch to local workspace" })).toHaveTextContent("Directory");
    expect(root.at("hittable/env.json")).toBeDefined();

    await user.click(toLocal());
    expect(screen.getByRole("button", { name: "Open a directory workspace" })).toBeInTheDocument();
    expect(localStorage.getItem("hittable-workspace-mode")).toBe("local");
  });

  it("without the File System Access API it explains which browsers work", async () => {
    const user = await mount();
    await user.click(screen.getByRole("button", { name: "Open a directory workspace" }));
    expect(screen.getByRole("status")).toHaveTextContent("Browser Not Supported");
    expect(screen.getByRole("status")).toHaveTextContent(/Chromium-based browsers/);
    expect(screen.getByRole("button", { name: "Open a directory workspace" })).toHaveTextContent("Local");
  });

  it("the hint also shows on focus and Escape hides it", async () => {
    const user = await mount();
    await user.tab();
    expect(screen.getByRole("status")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});
