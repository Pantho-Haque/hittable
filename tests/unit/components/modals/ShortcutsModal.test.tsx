// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ShortcutsModal from "@/components/modals/ShortcutsModal";
import { SHORTCUTS, SHORTCUT_GROUPS } from "@/constants/shortcuts";
import { openTool } from "@/utils/uiEvents";

describe("ShortcutsModal", () => {
  it("opens with ? and lists every shortcut under its group; Esc closes it", async () => {
    const user = userEvent.setup();
    render(<ShortcutsModal />);
    await user.keyboard("?");
    const d = screen.getByRole("dialog", { name: "Keyboard shortcuts" });
    for (const group of SHORTCUT_GROUPS) {
      const section = within(d).getByRole("region", { name: group });
      for (const s of SHORTCUTS.filter((x) => x.group === group)) {
        expect(within(section).getByText(s.description)).toBeInTheDocument();
      }
    }
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("ignores ? while typing, but Ctrl+/ opens it anywhere", async () => {
    const user = userEvent.setup();
    render(<><input aria-label="field" /><ShortcutsModal /></>);
    await user.click(screen.getByRole("textbox", { name: "field" }));
    await user.keyboard("?");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "field" })).toHaveValue("?");
    await user.keyboard("{Control>}/{/Control}");
    expect(screen.getByRole("dialog", { name: "Keyboard shortcuts" })).toBeInTheDocument();
  });

  it("opens from openTool('shortcuts')", () => {
    render(<ShortcutsModal />);
    act(() => { openTool("shortcuts"); });
    expect(screen.getByRole("dialog", { name: "Keyboard shortcuts" })).toBeInTheDocument();
  });
});
