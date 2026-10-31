// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import CommandPalette from "@/components/palette/CommandPalette";
import { DataProvider } from "@/context/dataContext";
import { NotificationProvider } from "@/context/notifyContext";
import { ShortcutProvider } from "@/context/ShortcutKeypressProvider";
import { sampleCollections } from "../../helpers/dataHarness";
import { openTool } from "@/utils/uiEvents";

const nav = vi.hoisted(() => {
  let url = "/hittable";
  const subs = new Set<() => void>();
  return {
    get: () => url,
    push: (u: string) => { url = u; subs.forEach((f) => f()); },
    reset: (u = "/hittable") => { url = u; },
    subscribe: (f: () => void) => { subs.add(f); return () => { subs.delete(f); }; },
  };
});
vi.mock("next/navigation", async () => {
  const React = await import("react");
  return {
    useRouter: () => ({ push: nav.push }),
    useSearchParams: () => {
      const url = React.useSyncExternalStore(nav.subscribe, nav.get);
      return React.useMemo(() => new URLSearchParams(url.split("?")[1] ?? ""), [url]);
    },
  };
});

// Local mode; the folder workspace is not under test here.
vi.mock("@/context/workspaceContext", () => ({
  useWorkspace: () => ({
    mode: "local", tree: [], activeFile: null,
    setActiveFile: () => {}, openFolder: async () => {}, disconnectFolder: () => {},
  }),
}));

function mount(url = "/hittable?c=Users&r=List%20users") {
  nav.reset(url);
  localStorage.setItem("hittable", JSON.stringify(sampleCollections()));
  render(
    <NotificationProvider>
      <DataProvider>
        <ShortcutProvider>
          <input aria-label="Request URL" />
          <CommandPalette />
        </ShortcutProvider>
      </DataProvider>
    </NotificationProvider>,
  );
  return userEvent.setup();
}

const palette = () => screen.findByRole("dialog", { name: "Command palette" });
const options = (d: HTMLElement) => within(d).getAllByRole("option").map((o) => o.textContent ?? "");

describe("CommandPalette", () => {
  beforeEach(() => localStorage.clear());

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
  // appearing used to land in whatever had focus (the URL bar), so a quick
  // "Ctrl+K env Enter" searched for nothing.
  it("keeps what is typed while it is still loading", async () => {
    mount();
    const input = screen.getByRole("textbox", { name: "Request URL" });
    const before = (input as HTMLTextAreaElement).value;
    input.focus();
    act(() => {
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "k", ctrlKey: true, bubbles: true, cancelable: true }));
      for (const key of ["e", "n", "x", "Backspace", "v"]) {
        input.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true }));
      }
    });
    const d = await palette();
    expect(within(d).getByRole("combobox")).toHaveValue("env");
    expect(input).toHaveValue(before); // nothing leaked into the URL bar
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
    expect(await screen.findByRole("dialog", { name: /Environment: Users/ })).toBeInTheDocument();
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

  it("filters requests across collections and folders, showing method and path", async () => {
    const user = mount();
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    const d = await palette();
    await user.keyboard("create us");
    const found = options(d);
    expect(found[0]).toContain("Create user");
    expect(found[0]).toContain("POST");
    expect(found[0]).toContain("Users / Admin");
    expect(found.join("|")).not.toContain("List users");
  });

  it("Enter runs the selected request, navigating like the sidebar does", async () => {
    const user = mount("/hittable");
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    const d = await palette();
    await user.keyboard("create user");
    const box = within(d).getByRole("combobox");
    expect(box.getAttribute("aria-activedescendant")).toBe(within(d).getAllByRole("option")[0].id);
    await user.keyboard("{Enter}");
    expect(screen.queryByRole("dialog", { name: "Command palette" })).not.toBeInTheDocument();
    await vi.waitFor(() => expect(nav.get()).toBe("/hittable?c=Users&r=Create%20user&p=Admin"));
  });

  it("arrow keys move the selection and the run item is remembered as recent", async () => {
    const user = mount("/hittable");
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    let d = await palette();
    await user.keyboard("user");
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

  it('typing "env" and Enter opens the Environment dialog of the open collection', async () => {
    const user = mount();
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    const d = await palette();
    await user.keyboard("env");
    expect(options(d)[0]).toContain("Environment variables");
    await user.keyboard("{Enter}");
    expect(await screen.findByRole("dialog", { name: "Environment: Users" })).toBeInTheDocument();
  });

  it("action rows show their shortcut from the registry", async () => {
    const user = mount();
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    const d = await palette();
    await user.keyboard("toggle sidebar");
    const row = within(d).getAllByRole("option")[0];
    expect(row).toHaveTextContent("Toggle sidebar");
    expect(within(row).getByText("B")).toBeInTheDocument();
  });

  it("still works when storage is blocked", async () => {
    const user = mount("/hittable");
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("blocked"); });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("blocked"); });
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    await palette();
    await user.keyboard("notes{Enter}");
    expect(screen.queryByRole("dialog", { name: "Command palette" })).not.toBeInTheDocument();
  });
});
