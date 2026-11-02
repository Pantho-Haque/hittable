import { describe, it, expect } from "vitest";
import { SHORTCUTS, SHORTCUT_GROUPS, shortcutById } from "@/constants/shortcuts";

const keys = (id: string) => shortcutById(id)?.keys.join("+");

describe("shortcut registry", () => {
  it("has unique ids and only known groups", () => {
    expect(new Set(SHORTCUTS.map((s) => s.id)).size).toBe(SHORTCUTS.length);
    for (const s of SHORTCUTS) expect(SHORTCUT_GROUPS).toContain(s.group);
  });

  it("lists every binding the app handles, with its keys", () => {
    expect(keys("palette")).toBe("Ctrl/Cmd+K");
    expect(keys("cheatsheet")).toBe("?");
    expect(keys("cheatsheet-alt")).toBe("Ctrl/Cmd+/");
    expect(keys("send")).toBe("Ctrl/Cmd+Enter");
    expect(keys("save")).toBe("Ctrl/Cmd+S");
    expect(keys("beautify")).toBe("Shift+Alt+F");
    expect(keys("cancel")).toBe("Esc");
    expect(keys("toggle-sidebar")).toBe("Ctrl/Cmd+B");
    expect(keys("search-response")).toBe("Ctrl/Cmd+F");
    expect(keys("wrap")).toBe("Alt+Z");
    expect(keys("source-goto-line")).toBe("Ctrl/Cmd+Alt+G");
    expect(keys("pane-collapse")).toBe("Enter");
  });

  it("binds nothing to a core browser shortcut", () => {
    // Ctrl+J is Downloads; Ctrl+Shift+C is DevTools' inspector on Windows/Linux.
    const all = SHORTCUTS.map((s) => s.keys.join("+"));
    for (const banned of ["Ctrl/Cmd+J", "Ctrl/Cmd+Shift+C", "Ctrl/Cmd+T", "Ctrl/Cmd+W", "Ctrl/Cmd+N", "Ctrl/Cmd+L"]) {
      expect(all).not.toContain(banned);
    }
  });

  it("groups the explorer toggle under Explorer", () => {
    expect(shortcutById("toggle-sidebar")?.group).toBe("Explorer");
  });
});
