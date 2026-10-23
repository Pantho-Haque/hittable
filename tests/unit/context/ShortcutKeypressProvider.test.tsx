// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ShortcutProvider, useShortcuts } from "@/context/ShortcutKeypressProvider";

function Sidebar() {
  const { shortcuts, toggle } = useShortcuts();
  return (
    <div>
      <span>{shortcuts.toggleSidebar ? "collapsed" : "expanded"}</span>
      <button onClick={() => toggle("toggleSidebar")}>toggle</button>
      <input aria-label="url" />
    </div>
  );
}

const mount = () => render(<ShortcutProvider><Sidebar /></ShortcutProvider>);

beforeEach(() => localStorage.clear());

describe("ShortcutProvider (sidebar toggle)", () => {
  it("the sidebar starts expanded on a first visit", () => {
    mount();
    expect(screen.getByText("expanded")).toBeInTheDocument();
  });

  it("Ctrl/Cmd+B toggles the sidebar, even while typing in a field", async () => {
    mount();
    const user = userEvent.setup();
    await user.keyboard("{Control>}b{/Control}");
    expect(screen.getByText("collapsed")).toBeInTheDocument();
    await user.click(screen.getByLabelText("url"));
    await user.keyboard("{Meta>}b{/Meta}");
    expect(screen.getByText("expanded")).toBeInTheDocument();
  });

  it("the toggle button works and the choice survives a reload", async () => {
    const { unmount } = mount();
    await userEvent.setup().click(screen.getByText("toggle"));
    expect(localStorage.getItem("hittable_sidebar_collapsed")).toBe("true");
    unmount();
    mount();
    expect(screen.getByText("collapsed")).toBeInTheDocument();
  });
});
