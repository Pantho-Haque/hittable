"use client";
import { useEffect, useRef, useState } from "react";
import { MergeView, unifiedMergeView } from "@codemirror/merge";
import { Compartment, EditorState, type Extension } from "@codemirror/state";
import { EditorView, lineNumbers } from "@codemirror/view";
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
});

export type DiffSides = { original: string | null; modified: string | null; originalLabel: string; modifiedLabel: string };

/** Read-only diff of two texts: inline by default, side by side on request. */
export function DiffEditor({ path, original, modified }: { path: string; original: string; modified: string }) {
  const [split, setSplit] = useState(false);
  const host = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const parent = host.current;
    if (!parent) return;
    const language = new Compartment();
    const base: Extension[] = [
      lineNumbers(),
      hittableTheme,
      diffTheme,
      EditorState.readOnly.of(true),
      EditorView.editable.of(false),
      language.of([]),
    ];
    const collapse = { margin: 3, minSize: 6 };
    let views: EditorView[];
    let destroy: () => void;
    if (split) {
      const mv = new MergeView({
        a: { doc: original, extensions: [...base, EditorView.contentAttributes.of({ "aria-label": `${path}, before` })] },
        b: { doc: modified, extensions: [...base, EditorView.contentAttributes.of({ "aria-label": `${path}, after` })] },
        parent,
        collapseUnchanged: collapse,
      });
      views = [mv.a, mv.b];
      destroy = () => mv.destroy();
    } else {
      const view = new EditorView({
        parent,
        state: EditorState.create({
          doc: modified,
          extensions: [
            ...base,
            unifiedMergeView({ original, mergeControls: false, collapseUnchanged: collapse }),
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
  }, [split, path, original, modified]);

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

/** The diff in a large dialog. `sides` null while the texts load. */
export default function DiffDialog({ path, sides, error, onClose }: { path: string; sides: DiffSides | null; error?: string | null; onClose: () => void }) {
  return (
    <ModalShell title={path} size="lg" onClose={onClose}>
      {error ? (
        <p role="alert" className="text-sm text-(--danger)">{error}</p>
      ) : !sides ? (
        <p role="status" className="text-sm text-(--muted)">Loading the diff…</p>
      ) : (
        <>
          <p className="text-xs text-(--muted)">
            <span className="text-(--git-deleted)">−</span> {sides.originalLabel}
            {sides.original === null && " (no file)"} · <span className="text-(--git-added)">+</span> {sides.modifiedLabel}
            {sides.modified === null && " (deleted)"}
          </p>
          <DiffEditor path={path} original={sides.original ?? ""} modified={sides.modified ?? ""} />
        </>
      )}
      <div className="flex justify-end">
        <button type="button" className="workspace-button border border-(--border-strong)" onClick={onClose}>
          Close
        </button>
      </div>
    </ModalShell>
  );
}
