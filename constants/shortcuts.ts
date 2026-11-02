// The one list of web-app shortcuts. The palette hints, the cheatsheet, the
// Info dialog and the landing page all render from it.
export type ShortcutGroup = "General" | "Request" | "Response" | "Explorer" | "Editor" | "Layout";

export type Shortcut = {
  id: string;
  keys: string[];
  description: string;
  group: ShortcutGroup;
};

export const SHORTCUT_GROUPS: ShortcutGroup[] = ["General", "Request", "Response", "Explorer", "Editor", "Layout"];

export const SHORTCUTS: Shortcut[] = [
  { id: "palette", keys: ["Ctrl/Cmd", "K"], description: "Open command palette", group: "General" },
  { id: "cheatsheet", keys: ["?"], description: "Show keyboard shortcuts", group: "General" },
  { id: "cheatsheet-alt", keys: ["Ctrl/Cmd", "/"], description: "Show keyboard shortcuts (anywhere)", group: "General" },
  { id: "close", keys: ["Esc"], description: "Close modals", group: "General" },
  { id: "send", keys: ["Ctrl/Cmd", "Enter"], description: "Send request", group: "Request" },
  { id: "save", keys: ["Ctrl/Cmd", "S"], description: "Save the request", group: "Request" },
  { id: "cancel", keys: ["Esc"], description: "Cancel the running request", group: "Request" },
  { id: "search-response", keys: ["Ctrl/Cmd", "F"], description: "Search in response", group: "Response" },
  { id: "source-goto-line", keys: ["Ctrl/Cmd", "Alt", "G"], description: "Go to line in the HTML source", group: "Response" },
  { id: "toggle-sidebar", keys: ["Ctrl/Cmd", "B"], description: "Toggle explorer", group: "Explorer" },
  { id: "beautify", keys: ["Shift", "Alt", "F"], description: "Beautify JSON", group: "Editor" },
  { id: "wrap", keys: ["Alt", "Z"], description: "Toggle line wrap", group: "Editor" },
  // On a focused divider (Tab to it): the WAI-ARIA window splitter keys.
  { id: "pane-resize", keys: ["Arrow keys"], description: "Resize panes at a focused divider (Shift for bigger steps)", group: "Layout" },
  { id: "pane-collapse", keys: ["Enter"], description: "Collapse or restore the pane at a focused divider", group: "Layout" },
  { id: "pane-reset", keys: ["Double-click"], description: "Reset a divider to its default", group: "Layout" },
];

export const shortcutById = (id: string) => SHORTCUTS.find((s) => s.id === id);
