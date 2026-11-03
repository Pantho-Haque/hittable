"use client";
import { useEffect, useRef, useState } from "react";
import { MergeView, unifiedMergeView } from "@codemirror/merge";
import { Compartment, EditorState, type Extension } from "@codemirror/state";
import { EditorView, keymap, lineNumbers } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { ModalShell } from "@/components/ui/SharedModal";
import { hittableTheme } from "@/components/workspace/editorTheme";
import { languageForFile } from "@/utils/workspace/editorLanguages";

// Changed lines tinted with the git status tokens rather than the library's own red/green.
const diffTheme = EditorView.theme({
  ".cm-changedLine, .cm-insertedLine": { backgroundColor: "color-mix(in srgb, var(--git-added) 12%, transparent)" },
  ".cm-changedText, .cm-insertedText": { background: "color-mix(in srgb, var(--git-added) 30%, transparent)" },
  ".cm-deletedChunk, .cm-deletedLine": { backgroundColor: "color-mix(in srgb, var(--git-deleted) 12%, transparent)" },
  ".cm-deletedText, .cm-deletedChunk .cm-deletedText": { background: "color-mix(in srgb, var(--git-deleted) 30%, transparent)" },
  ".cm-collapsedLines": { color: "var(--muted)", background: "var(--surface-2)" },
  // Revert in the inline view (inside the editor; the split view's ⟲ is styled in workspace.css).
  ".cm-chunkButtons button": {
    color: "var(--muted)",
    border: "1px solid var(--border-strong)",
    borderRadius: "4px",
    background: "var(--surface-1)",
    padding: "0 6px",
    fontSize: "11px",
    cursor: "pointer",
  },
  ".cm-chunkButtons button:hover": { color: "var(--brand)" },
});

/** The ⟲ between the panes: puts the last commit's lines back over yours for that change. */
const revertButton = () => {
  const b = document.createElement("button");
  b.type = "button";
  b.textContent = "⟲";
  b.title = "Revert this change";
  b.setAttribute("aria-label", "Revert this change");
  return b;
};

/** Inline view: only Revert (Accept would rewrite the last commit, which isn't yours to edit). */
const inlineControls = (type: "accept" | "reject", action: (e: MouseEvent) => void) => {
  if (type === "accept") return document.createElement("span");
  const b = revertButton();
  b.textContent = "⟲ Revert";
  b.onmousedown = action;
  return b;
};

export type DiffSides = {
  original: string | null;
  modified: string | null;
  originalLabel: string;
  modifiedLabel: string;
  /** Byte sizes when either side is binary: no text diff then. */
  binary?: { original: number | null; modified: number | null } | null;
};

const size = (n: number | null) => (n === null ? "no file" : n >= 1024 * 1024 ? `${(n / 1024 / 1024).toFixed(1)} MB` : n >= 1024 ? `${(n / 1024).toFixed(1)} KB` : `${n} bytes`);

/**
 * Diff of two texts: side by side, or inline on request (and by default on a
 * phone). With `onChange` your side is editable and every change has a ⟲
 * that puts the last commit's lines back; each edit is reported as the whole text.
 */
export function DiffEditor({
  path,
  original,
  modified,
  onChange,
}: {
  path: string;
  original: string;
  modified: string;
  onChange?: (text: string) => void;
}) {
  const [split, setSplit] = useState(() => typeof window === "undefined" || window.innerWidth >= 768);
  const host = useRef<HTMLDivElement>(null);
  const editable = !!onChange;
  // Your text as last typed: a layout switch rebuilds the views from it, and
  // the views are never rebuilt for a `modified` that is just your own edit coming back.
  const mine = useRef(modified);
  const change = useRef(onChange);
  useEffect(() => {
    change.current = onChange;
  });

  useEffect(() => {
    const parent = host.current;
    if (!parent) return;
    const language = new Compartment();
    const readOnly = [EditorState.readOnly.of(true), EditorView.editable.of(false)];
    const base: Extension[] = [lineNumbers(), hittableTheme, diffTheme, language.of([])];
    const yours: Extension[] = editable
      ? [
          history(),
          keymap.of([...defaultKeymap, ...historyKeymap]),
          EditorView.updateListener.of((u) => {
            if (!u.docChanged) return;
            mine.current = u.state.doc.toString();
            change.current?.(mine.current);
          }),
        ]
      : readOnly;
    const collapse = { margin: 3, minSize: 6 };
    let views: EditorView[];
    let destroy: () => void;
    if (split) {
      const mv = new MergeView({
        a: { doc: original, extensions: [...base, ...readOnly, EditorView.contentAttributes.of({ "aria-label": `${path}, before` })] },
        b: { doc: mine.current, extensions: [...base, ...yours, EditorView.contentAttributes.of({ "aria-label": `${path}, after` })] },
        parent,
        collapseUnchanged: collapse,
        ...(editable && { revertControls: "a-to-b" as const, renderRevertControl: revertButton }),
      });
      views = [mv.a, mv.b];
      destroy = () => mv.destroy();
    } else {
      const view = new EditorView({
        parent,
        state: EditorState.create({
          doc: mine.current,
          extensions: [
            ...base,
            ...yours,
            unifiedMergeView({ original, mergeControls: editable ? inlineControls : false, collapseUnchanged: collapse }),
            EditorView.contentAttributes.of({ "aria-label": `Changes to ${path}` }),
          ],
        }),
      });
      views = [view];
      destroy = () => view.destroy();
    }
    let live = true;
    void languageForFile(path.split("/").at(-1) ?? "")
      ?.load()
      .then((support) => live && views.forEach((v) => v.dispatch({ effects: language.reconfigure(support) })));
    return () => {
      live = false;
      destroy();
    };
  }, [split, path, original, editable]);

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2">
      <div role="group" aria-label="Diff layout" className="flex gap-1">
        <button type="button" className="workspace-button" aria-pressed={!split} onClick={() => setSplit(false)}>
          Inline
        </button>
        <button type="button" className="workspace-button" aria-pressed={split} onClick={() => setSplit(true)}>
          Side by side
        </button>
      </div>
      <div ref={host} className="min-h-0 flex-1 overflow-auto rounded-md border border-(--border) bg-(--ink-700) font-mono text-[13px]" />
    </div>
  );
}

/** The diff of one file: loading, error, binary sizes, or the text diff. */
export function DiffBody({
  path,
  sides,
  error,
  onChange,
}: {
  path: string;
  sides: DiffSides | null;
  error?: string | null;
  /** Makes your side editable (a text file that still exists). */
  onChange?: (text: string) => void;
}) {
  if (error) return <p role="alert" className="text-sm text-(--danger)">{error}</p>;
  if (!sides) return <p role="status" className="text-sm text-(--muted)">Loading the diff…</p>;
  if (sides.binary) {
    return (
      <div className="rounded-md border border-(--border) p-4 text-sm text-(--text)">
        <p className="mb-2 text-(--muted)">Binary file: there is no text to compare.</p>
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-xs">
          <dt className="text-(--git-deleted)">− {sides.originalLabel}</dt>
          <dd>{size(sides.binary.original)}</dd>
          <dt className="text-(--git-added)">+ {sides.modifiedLabel}</dt>
          <dd>{sides.binary.modified === null ? "deleted" : size(sides.binary.modified)}</dd>
        </dl>
      </div>
    );
  }
  return (
    <>
      <p className="text-xs text-(--muted)">
        <span className="text-(--git-deleted)">−</span> {sides.originalLabel}
        {sides.original === null && " (no file)"} · <span className="text-(--git-added)">+</span> {sides.modifiedLabel}
        {sides.modified === null && " (deleted)"}
        {onChange && sides.modified !== null && " · edit your side; ⟲ reverts one change, Ctrl/Cmd+Z undoes"}
      </p>
      <DiffEditor
        // A new file or a new base starts over from the stored text; your own
        // edits coming back as `modified` never rebuild the views.
        key={`${path}\0${sides.original ?? ""}`}
        path={path}
        original={sides.original ?? ""}
        modified={sides.modified ?? ""}
        onChange={sides.modified === null ? undefined : onChange}
      />
    </>
  );
}

/** The diff in a large dialog (comparing with theirs). `sides` null while the texts load. */
export default function DiffDialog({ path, sides, error, onClose }: { path: string; sides: DiffSides | null; error?: string | null; onClose: () => void }) {
  return (
    <ModalShell title={path} size="lg" onClose={onClose}>
      <DiffBody path={path} sides={sides} error={error} />
      <div className="flex justify-end">
        <button type="button" className="workspace-button border border-(--border-strong)" onClick={onClose}>
          Close
        </button>
      </div>
    </ModalShell>
  );
}
