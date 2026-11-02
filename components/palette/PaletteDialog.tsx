"use client";

import {
  useEffect, useId, useMemo, useState, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import { Command, FileText } from "lucide-react";
import { ModalShell } from "@/components/ui/SharedModal";
import { ShortcutKeys } from "@/components/modals/ShortcutsModal";
import { shortcutById } from "@/constants/shortcuts";
import { useWorkspace } from "@/context/workspaceContext";
import { useShortcuts } from "@/context/ShortcutKeypressProvider";
import { fuzzyScore } from "@/utils/fuzzy";
import { openTool, requestCopyCurl, type ToolName } from "@/utils/uiEvents";
import type { WsNode } from "@/services/workspace/backend";

type Item = {
  id: string;
  kind: "action" | "file";
  label: string;
  detail?: string;
  shortcut?: string;
  keywords?: string;
  run: () => void;
};

const RECENT_KEY = "hittable_palette_recent";
const MAX_RECENT = 8;
const MAX_SHOWN = 50;

function loadRecent(): string[] {
  try {
    const v = JSON.parse(localStorage.getItem(RECENT_KEY) ?? "[]");
    return Array.isArray(v) ? v.filter((x) => typeof x === "string") : [];
  } catch {
    return []; // storage blocked or corrupt
  }
}

function saveRecent(id: string) {
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify([id, ...loadRecent().filter((x) => x !== id)].slice(0, MAX_RECENT)));
  } catch {
    // storage blocked: recents are a convenience
  }
}

function walkFiles(nodes: WsNode[], visit: (node: WsNode) => void) {
  for (const node of nodes) {
    if (node.kind === "file") visit(node);
    else if (node.children) walkFiles(node.children, visit);
  }
}

const noSubscribe = () => () => {};
const isMac = () => /Mac|iPhone|iPad/.test(navigator.platform);
const directorySupported = () => typeof window !== "undefined" && "showDirectoryPicker" in window;

export default function PaletteDialog({
  onClose,
  typedSoFar = () => "",
  takeTyped = () => "",
}: {
  onClose: () => void;
  /** Characters typed since Ctrl+K while this dialog was loading (read-only). */
  typedSoFar?: () => string;
  /** The same, and stop holding keys: from now on they reach the input. */
  takeTyped?: () => string;
}) {
  const { backendKind, tree, setActiveFile, activeFile, openFolder, closeWorkspace } = useWorkspace();
  const router = useRouter();
  const { toggle } = useShortcuts();
  // Render reads what was typed while loading (render may run twice); the
  // effect takes the final text, keys typed until then included.
  // A trailing newline means Enter was pressed while loading: run the top
  // result once the query is in.
  const [query, setQuery] = useState(() => typedSoFar().split("\n")[0]);
  const [runPending, setRunPending] = useState(false);
  useEffect(() => {
    const all = takeTyped();
    if (!all) return;
    const [text] = all.split("\n");
    setQuery(text); // eslint-disable-line react-hooks/set-state-in-effect -- one-off handover from the opener
    if (all.includes("\n")) setRunPending(true);
  }, [takeTyped]);
  const [active, setActive] = useState(0);
  const [recent] = useState(loadRecent);
  const listId = useId();
  const mac = useSyncExternalStore(noSubscribe, isMac, () => false);

  const items = useMemo<Item[]>(() => {
    const tool = (id: ToolName, label: string, keywords: string, target?: string, shortcut?: string): Item => ({
      id: `action:${id}`, kind: "action", label, keywords, shortcut, run: () => openTool(id, target),
    });
    const list: Item[] = [];
    list.push({
      id: "action:toggle-sidebar", kind: "action", label: "Toggle explorer",
      keywords: "hide show panel sidebar", shortcut: "toggle-sidebar", run: () => toggle("toggleSidebar"),
    });
    if (activeFile?.kind === "hit") {
      list.push({ id: "action:copy-curl", kind: "action", label: "Copy request as cURL", keywords: "clipboard export", run: requestCopyCurl });
    }
    list.push(tool("shortcuts", "Keyboard shortcuts", "keys cheatsheet hotkeys help", undefined, "cheatsheet"));
    if (backendKind) {
      list.push({
        id: "action:close-folder", kind: "action", label: backendKind === "folder" ? "Close folder" : "Close repository",
        keywords: "disconnect workspace", run: () => { closeWorkspace(); router.push("/hittable"); },
      });
    }
    if (directorySupported()) {
      list.push({
        id: "action:open-folder", kind: "action", label: "Open a local folder", keywords: "directory workspace disk",
        run: () => void openFolder().then((ok) => ok && router.push("/hittable/folder")),
      });
    }
    walkFiles(tree, (node) => {
      list.push({
        id: `file:${node.path.join("/")}`,
        kind: "file",
        label: node.name,
        detail: node.path.slice(0, -1).join(" / "),
        run: () => setActiveFile(node.path),
      });
    });
    return list;
  }, [backendKind, activeFile, tree, toggle, setActiveFile, openFolder, closeWorkspace, router]);

  const results = useMemo(() => {
    const rank = (id: string) => {
      const i = recent.indexOf(id);
      return i < 0 ? MAX_RECENT : i;
    };
    if (!query.trim()) {
      // Recently used first, then actions, then files in their order.
      return items
        .map((item, i) => ({ item, i }))
        .sort((a, b) => rank(a.item.id) - rank(b.item.id) || a.i - b.i)
        .slice(0, MAX_SHOWN)
        .map((x) => x.item);
    }
    const scored: { item: Item; score: number }[] = [];
    for (const item of items) {
      const own = fuzzyScore(query, item.label);
      const score = own !== null ? own + 50 : fuzzyScore(query, `${item.label} ${item.detail ?? ""} ${item.keywords ?? ""}`);
      if (score !== null) scored.push({ item, score: score + (MAX_RECENT - rank(item.id)) * 2 });
    }
    return scored.sort((a, b) => b.score - a.score).slice(0, MAX_SHOWN).map((x) => x.item);
  }, [items, query, recent]);

  const current = Math.min(active, results.length - 1);
  const optionId = (i: number) => `${listId}-opt-${i}`;

  const run = (item: Item | undefined) => {
    if (!item) return;
    saveRecent(item.id);
    onClose();
    // After the palette has closed and handed focus back, so a dialog the
    // action opens returns focus to the same place.
    setTimeout(item.run);
  };

  useEffect(() => {
    if (!runPending) return;
    setRunPending(false); // eslint-disable-line react-hooks/set-state-in-effect -- consumes the held Enter once
    run(results[0]);
  }, [runPending]); // eslint-disable-line react-hooks/exhaustive-deps -- runs once, with the results of the handed-over query

  const onKeyDown = (e: React.KeyboardEvent) => {
    const n = results.length;
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (n) setActive((current + (e.key === "ArrowDown" ? 1 : n - 1)) % n);
    } else if (e.key === "Home" || e.key === "End") {
      e.preventDefault();
      setActive(e.key === "Home" ? 0 : Math.max(n - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      run(results[current]);
    }
  };

  const scrollIntoView = (el: HTMLLIElement | null) => el?.scrollIntoView?.({ block: "nearest" });

  return (
    <ModalShell title="Command palette" onClose={onClose} size="wide">
      <div className="flex min-h-0 flex-col gap-3 font-sans">
        <input
          autoFocus
          type="text"
          role="combobox"
          aria-expanded="true"
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={current >= 0 ? optionId(current) : undefined}
          aria-label="Search requests, files and actions"
          placeholder="Search files and actions…"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setActive(0);
          }}
          onKeyDown={onKeyDown}
          className="w-full rounded-md border border-white/10 bg-(--surface-0) px-3 py-2 text-sm text-white/90 placeholder-white/50 outline-none focus:border-(--brand)/60"
        />
        <ul id={listId} role="listbox" aria-label="Results" className="-mx-2 max-h-[50dvh] min-h-0 overflow-y-auto">
          {results.map((item, i) => {
            const selected = i === current;
            const keys = item.shortcut ? shortcutById(item.shortcut)?.keys : undefined;
            return (
              <li
                key={item.id}
                id={optionId(i)}
                ref={selected ? scrollIntoView : undefined}
                role="option"
                aria-selected={selected}
                onMouseMove={() => !selected && setActive(i)}
                onMouseDown={(e) => e.preventDefault()} // keep focus in the input
                onClick={() => run(item)}
                className={`flex min-h-11 cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm md:min-h-9 ${
                  selected ? "bg-(--surface-2) text-white" : "text-white/75"
                }`}
              >
                {item.kind === "file" ? (
                  <FileText size={14} className="mx-5 shrink-0 text-(--muted)" aria-hidden="true" />
                ) : (
                  <Command size={14} className="mx-5 shrink-0 text-(--brand)" aria-hidden="true" />
                )}
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate">{item.label}</span>
                  {item.detail && <span className="truncate text-xs text-white/60">{item.detail}</span>}
                </span>
                {keys && (
                  <span className="hidden sm:flex">
                    <ShortcutKeys keys={keys.map((k) => (k === "Ctrl/Cmd" ? (mac ? "⌘" : "Ctrl") : k))} />
                  </span>
                )}
              </li>
            );
          })}
          {results.length === 0 && (
            <li role="option" aria-selected="false" aria-disabled="true" className="px-2 py-3 text-sm text-white/60">
              No matches
            </li>
          )}
        </ul>
        <p className="hidden text-xs text-white/60 sm:block" aria-hidden="true">
          ↑↓ to move · Enter to run · Esc to close
        </p>
      </div>
    </ModalShell>
  );
}
