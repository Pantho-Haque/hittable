"use client";

import { useEffect, useState } from "react";
import { ModalShell } from "@/components/ui/SharedModal";
import { SHORTCUTS, SHORTCUT_GROUPS } from "@/constants/shortcuts";
import { useToolListener } from "@/utils/uiEvents";

export function ShortcutKeys({ keys }: { keys: string[] }) {
  return (
    <span className="flex shrink-0 items-center gap-1">
      {keys.map((k, i) => (
        <span key={k} className="flex items-center gap-1">
          <kbd className="kbd">{k}</kbd>
          {i < keys.length - 1 && <span className="text-[11px] text-white/60" aria-hidden="true">+</span>}
        </span>
      ))}
    </span>
  );
}

const isTyping = (el: EventTarget | null) =>
  el instanceof HTMLElement &&
  (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.tagName === "SELECT" || el.isContentEditable);

/** The cheatsheet: `?` outside text fields, `Ctrl/Cmd+/` anywhere, or openTool("shortcuts"). */
export default function ShortcutsModal() {
  const [open, setOpen] = useState(false);
  useToolListener("shortcuts", () => setOpen(true));

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const slash = (e.ctrlKey || e.metaKey) && !e.altKey && (e.key === "/" || e.code === "Slash");
      const question = e.key === "?" && !e.ctrlKey && !e.metaKey && !e.altKey && !isTyping(e.target);
      if (!slash && !question) return;
      e.preventDefault();
      // Capture phase: the editor would otherwise take Ctrl+/ as "toggle comment".
      e.stopPropagation();
      setOpen(true);
    };
    window.addEventListener("keydown", handler, true);
    return () => window.removeEventListener("keydown", handler, true);
  }, []);

  if (!open) return null;
  return (
    <ModalShell title="Keyboard shortcuts" onClose={() => setOpen(false)} size="wide">
      {/* Focusable so keyboard users can scroll it on short screens. */}
      <div
        tabIndex={0}
        className="-mr-2 flex min-h-0 flex-col gap-4 overflow-y-auto rounded-md pr-2 font-sans outline-none focus-visible:ring-2 focus-visible:ring-(--brand)/60"
      >
        {SHORTCUT_GROUPS.map((group) => (
          <section key={group} aria-labelledby={`shortcuts-${group}`}>
            <h3 id={`shortcuts-${group}`} className="mb-1.5 text-[11px] font-semibold uppercase tracking-[0.2em] text-(--brand)">
              {group}
            </h3>
            <ul className="flex flex-col divide-y divide-white/5">
              {SHORTCUTS.filter((s) => s.group === group).map((s) => (
                <li key={s.id} className="flex items-center justify-between gap-3 py-1.5 text-xs text-white/75">
                  <span>{s.description}</span>
                  <ShortcutKeys keys={s.keys} />
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </ModalShell>
  );
}
