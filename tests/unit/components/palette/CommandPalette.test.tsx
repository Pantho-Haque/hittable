// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import CommandPalette from "@/components/palette/CommandPalette";
import { ShortcutProvider } from "@/context/ShortcutKeypressProvider";
import { openTool } from "@/utils/uiEvents";
import type { WsNode } from "@/services/workspace/backend";

const push = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

const file = (...path: string[]): WsNode => ({ name: path[path.length - 1], path, kind: "file" });
const dir = (children: WsNode[], ...path: string[]): WsNode => ({ name: path[path.length - 1], path, kind: "directory", children });
const tree: WsNode[] = [
  dir([
    file("hittable", "env.json"),
    dir([file("hittable", "users", "list.hit"), file("hittable", "users", "create.hit")], "hittable", "users"),
  ], "hittable"),
  file("README.md"),
];

const ws = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
vi.mock("@/context/workspaceContext", () => ({ useWorkspace: () => ws.current }));

function mount() {
  render(
    <ShortcutProvider>
      <input aria-label="Request URL" />
      <CommandPalette />
    </ShortcutProvider>,
  );
  return userEvent.setup();
}

const palette = () => screen.findByRole("dialog", { name: "Command palette" });
const options = (d: HTMLElement) => within(d).getAllByRole("option").map((o) => o.textContent ?? "");
const fn = (name: string) => ws.current[name] as ReturnType<typeof vi.fn>;

beforeEach(() => {
  localStorage.clear();
  push.mockReset();
  ws.current = {
    backendKind: "folder", tree, activeFile: null,
    setActiveFile: vi.fn(), openFolder: vi.fn(async () => true), closeWorkspace: vi.fn(),
  };
});
afterEach(() => {
  delete (window as Partial<Window>).showDirectoryPicker;
});

describe("CommandPalette", () => {
  it("opens on Ctrl+K even while an input has focus, and Esc closes it and gives focus back", async () => {
    const user = mount();
    const input = screen.getByRole("textbox", { name: "Request URL" });
    input.focus();
    const event = new KeyboardEvent("keydown", { key: "k", ctrlKey: true, bubbles: true, cancelable: true });
    act(() => { input.dispatchEvent(event); });
    expect(event.defaultPrevented).toBe(true);
    const d = await palette();
    const box = within(d).getByRole("combobox");
    expect(box).toHaveFocus();
    expect(box).toHaveAttribute("aria-controls", within(d).getByRole("listbox").id);

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(input).toHaveFocus();
  });

  // The dialog is loaded lazily; keys typed between Ctrl+K and its input
  // appearing used to land in whatever had focus (the URL bar).
  it("keeps what is typed while it is still loading", async () => {
    mount();
    const input = screen.getByRole("textbox", { name: "Request URL" });
    input.focus();
    act(() => {
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "k", ctrlKey: true, bubbles: true, cancelable: true }));
      for (const key of ["e", "n", "x", "Backspace", "v"]) {
        input.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true }));
      }
    });
    const d = await palette();
    expect(within(d).getByRole("combobox")).toHaveValue("env");
    expect(input).toHaveValue(""); // nothing leaked into the URL bar
  });

  it("an Enter pressed while it loads runs the top result", async () => {
    mount();
    const input = screen.getByRole("textbox", { name: "Request URL" });
    input.focus();
    act(() => {
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "k", ctrlKey: true, bubbles: true, cancelable: true }));
      for (const key of [..."env", "Enter"]) {
        input.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true }));
      }
    });
    await vi.waitFor(() => expect(fn("setActiveFile")).toHaveBeenCalledWith(["hittable", "env.json"]));
  });

  it("also opens with Meta+K (Mac) and from openTool('palette')", async () => {
    mount();
    fireEvent.keyDown(window, { key: "k", metaKey: true });
    expect(await palette()).toBeInTheDocument();
    fireEvent.keyDown(window, { key: "k", metaKey: true }); // toggles closed
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    act(() => { openTool("palette"); });
    expect(await palette()).toBeInTheDocument();
  });

  it("finds files anywhere in the tree, showing the folder they are in", async () => {
    const user = mount();
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    const d = await palette();
    await user.keyboard("create");
    const found = options(d);
    expect(found[0]).toContain("create.hit");
    expect(found[0]).toContain("hittable / users");
    expect(found.join("|")).not.toContain("README.md");
  });

  it("Enter opens the selected file by its path", async () => {
    const user = mount();
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    const d = await palette();
    await user.keyboard("list.hit");
    const box = within(d).getByRole("combobox");
    expect(box.getAttribute("aria-activedescendant")).toBe(within(d).getAllByRole("option")[0].id);
    await user.keyboard("{Enter}");
    expect(screen.queryByRole("dialog", { name: "Command palette" })).not.toBeInTheDocument();
    await vi.waitFor(() => expect(fn("setActiveFile")).toHaveBeenCalledWith(["hittable", "users", "list.hit"]));
  });

  it("arrow keys move the selection and the run item is remembered as recent", async () => {
    const user = mount();
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    let d = await palette();
    await user.keyboard(".hit");
    const box = within(d).getByRole("combobox");
    const opts = within(d).getAllByRole("option");
    await user.keyboard("{ArrowDown}");
    expect(box.getAttribute("aria-activedescendant")).toBe(opts[1].id);
    expect(opts[1]).toHaveAttribute("aria-selected", "true");
    const second = opts[1].textContent!;
    await user.keyboard("{Enter}");

    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    d = await palette();
    expect(options(d)[0]).toBe(second); // empty query: recent first
  });

  it("Close folder closes the workspace and returns to the launcher", async () => {
    const user = mount();
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    await palette();
    await user.keyboard("close folder{Enter}");
    await vi.waitFor(() => expect(fn("closeWorkspace")).toHaveBeenCalled());
    expect(push).toHaveBeenCalledWith("/hittable");
  });

  it("Open a local folder is offered where the browser supports it, and opens the folder workspace", async () => {
    window.showDirectoryPicker = vi.fn() as unknown as typeof window.showDirectoryPicker;
    ws.current = { ...ws.current, backendKind: null, tree: [] };
    const user = mount();
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    const d = await palette();
    expect(options(d).join("|")).not.toContain("Close folder");
    await user.keyboard("open a local folder{Enter}");
    await vi.waitFor(() => expect(push).toHaveBeenCalledWith("/hittable/folder"));
    expect(fn("openFolder")).toHaveBeenCalled();
  });

  it("without File System Access there is no Open a local folder", async () => {
    mount();
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    const d = await palette();
    expect(options(d).join("|")).not.toContain("Open a local folder");
  });

  it("Copy request as cURL is offered only with a .hit file open", async () => {
    ws.current.activeFile = { path: ["hittable", "users", "list.hit"], kind: "hit" };
    mount();
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(options(await palette()).join("|")).toContain("Copy request as cURL");
  });

  it("action rows show their shortcut from the registry", async () => {
    const user = mount();
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    const d = await palette();
    await user.keyboard("toggle explorer");
    const row = within(d).getAllByRole("option")[0];
    expect(row).toHaveTextContent("Toggle explorer");
    expect(within(row).getByText("B")).toBeInTheDocument();
  });

  it("still works when storage is blocked", async () => {
    const user = mount();
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("blocked"); });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("blocked"); });
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    await palette();
    await user.keyboard("readme{Enter}");
    expect(screen.queryByRole("dialog", { name: "Command palette" })).not.toBeInTheDocument();
    await vi.waitFor(() => expect(fn("setActiveFile")).toHaveBeenCalledWith(["README.md"]));
    vi.restoreAllMocks();
  });
});
