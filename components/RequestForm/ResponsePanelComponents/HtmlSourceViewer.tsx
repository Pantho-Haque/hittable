"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronsDownUp, ChevronsUpDown, Code2, Search, Sparkles, WrapText } from "lucide-react";
import { useStoredFlag } from "@/utils/prefs";
import { html } from "@codemirror/lang-html";
import { codeFolding, foldAll, foldGutter, foldKeymap, unfoldAll } from "@codemirror/language";
import { highlightSelectionMatches, openSearchPanel, search, searchKeymap } from "@codemirror/search";
import { Compartment, EditorState, StateEffect, StateField } from "@codemirror/state";
import { Decoration, EditorView, drawSelection, highlightActiveLine, highlightActiveLineGutter, keymap, lineNumbers } from "@codemirror/view";
import { defaultKeymap } from "@codemirror/commands";
import { hittableTheme } from "@/components/workspace/editorTheme";

const markLine = StateEffect.define<number | null>();
const pickedLine = StateField.define({
  create: () => Decoration.none,
  update(deco, tr) {
    for (const e of tr.effects) {
      if (!e.is(markLine)) continue;
      if (e.value == null || e.value >= tr.state.doc.lines) return Decoration.none;
      const line = tr.state.doc.line(e.value + 1);
      return Decoration.set([Decoration.line({ class: "cm-picked-line" }).range(line.from)]);
    }
    return deco.map(tr.changes);
  },
  provide: (f) => EditorView.decorations.from(f),
});

const pickedTheme = EditorView.theme({ ".cm-picked-line": { backgroundColor: "rgba(55, 212, 224, 0.14)" } });

/**
 * The HTML response's source, read-only, in the same editor as files: one
 * scroll area for the whole document (no scrollbar per line), only visible
 * lines drawn, tag folding, wrap toggle, Ctrl/Cmd+F search and
 * Ctrl/Cmd+Alt+G go-to-line. `highlightedLine` (0-based) is the element picked
 * in the preview; it is marked and scrolled into view.
 */
type Props = {
  source: string;
  highlightedLine?: number | null;
  /** Pretty-printed or as received; the panel owns it so inspect looks up the shown text. */
  pretty?: boolean;
  onPrettyChange?: (pretty: boolean) => void;
};

export default function HtmlSourceViewer({ source, highlightedLine, pretty, onPrettyChange }: Props) {
  const hostRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const wrapSlot = useRef(new Compartment());
  const [wrap, setWrap] = useStoredFlag("hittable:source-wrap", false);
  const [folded, setFolded] = useState(false);
  const [line, setLine] = useState(1);

  useEffect(() => {
    const view = new EditorView({
      parent: hostRef.current!,
      state: EditorState.create({
        doc: source,
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightActiveLine(),
          drawSelection(),
          codeFolding({ placeholderText: "⋯" }),
          foldGutter({ openText: "▾", closedText: "▸" }),
          html(),
          search({ top: true }),
          highlightSelectionMatches(),
          keymap.of([...searchKeymap, ...foldKeymap, ...defaultKeymap]),
          EditorState.readOnly.of(true),
          EditorView.contentAttributes.of({ "aria-label": "Response HTML source", "aria-readonly": "true" }),
          hittableTheme,
          pickedTheme,
          pickedLine,
          wrapSlot.current.of(wrap ? EditorView.lineWrapping : []),
          EditorView.updateListener.of((u) => {
            if (u.selectionSet || u.docChanged) setLine(u.state.doc.lineAt(u.state.selection.main.head).number);
          }),
        ],
      }),
    });
    viewRef.current = view;
    return () => {
      view.destroy();
      viewRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- wrap is seeded here, then toggled through its compartment
  }, [source]);

  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const effects: StateEffect<unknown>[] = [markLine.of(highlightedLine ?? null)];
    if (highlightedLine != null && highlightedLine < view.state.doc.lines) {
      const at = view.state.doc.line(highlightedLine + 1).from;
      effects.push(EditorView.scrollIntoView(at, { y: "center" }));
      view.dispatch({ effects, selection: { anchor: at } });
    } else {
      view.dispatch({ effects });
    }
  }, [highlightedLine, source]);

  const toggleWrap = () => {
    viewRef.current?.dispatch({ effects: wrapSlot.current.reconfigure(wrap ? [] : EditorView.lineWrapping) });
    setWrap(!wrap);
  };
  const toggleFold = () => {
    const view = viewRef.current;
    if (!view) return;
    if (folded) unfoldAll(view);
    else foldAll(view);
    setFolded(!folded);
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 items-center gap-1 border-b border-(--border) bg-(--surface-2) px-2 py-1">
        <Code2 className="h-3 w-3 text-(--muted)" aria-hidden="true" />
        <span className="text-xs font-semibold text-(--text)">Source</span>
        <span className="ml-2 font-mono text-[11px] text-(--muted)" aria-live="polite">Ln {line}</span>
        <div className="ml-auto flex items-center gap-0.5">
          <ToolButton label="Search in source (Ctrl/Cmd+F)" onClick={() => viewRef.current && openSearchPanel(viewRef.current)}>
            <Search className="h-3.5 w-3.5" aria-hidden="true" />
          </ToolButton>
          <ToolButton label={folded ? "Expand all tags" : "Collapse all tags"} pressed={folded} onClick={toggleFold}>
            {folded ? <ChevronsUpDown className="h-3.5 w-3.5" aria-hidden="true" /> : <ChevronsDownUp className="h-3.5 w-3.5" aria-hidden="true" />}
          </ToolButton>
          {onPrettyChange && (
            <ToolButton label={pretty ? "Pretty-printed: show as received" : "As received: pretty-print"} pressed={!!pretty} onClick={() => onPrettyChange(!pretty)}>
              <Sparkles className="h-3.5 w-3.5" aria-hidden="true" />
            </ToolButton>
          )}
          <ToolButton label="Wrap long lines" pressed={wrap} onClick={toggleWrap}>
            <WrapText className="h-3.5 w-3.5" aria-hidden="true" />
          </ToolButton>
        </div>
      </div>
      <div ref={hostRef} className="response-source min-h-0 flex-1 overflow-hidden" />
    </div>
  );
}

function ToolButton({ label, pressed, onClick, children }: { label: string; pressed?: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      aria-label={label}
      aria-pressed={pressed}
      title={label}
      onClick={onClick}
      className={`rounded p-1 transition-colors ${pressed ? "bg-(--accent-soft) text-(--brand)" : "text-(--muted) hover:bg-(--ink-600) hover:text-(--text)"}`}
    >
      {children}
    </button>
  );
}
