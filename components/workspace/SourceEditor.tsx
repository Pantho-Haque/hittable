"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ChevronsDownUp, ChevronsUpDown, FileCode2, WrapText } from "lucide-react";
import {
  autocompletion,
  closeBrackets,
  closeBracketsKeymap,
  completionKeymap,
} from "@codemirror/autocomplete";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import {
  bracketMatching,
  codeFolding,
  foldAll,
  foldGutter,
  foldKeymap,
  indentOnInput,
  indentUnit,
  unfoldAll,
} from "@codemirror/language";
import { highlightSelectionMatches, search, searchKeymap } from "@codemirror/search";
import { Annotation, Compartment, EditorState, Prec } from "@codemirror/state";
import {
  EditorView,
  drawSelection,
  highlightActiveLine,
  highlightActiveLineGutter,
  highlightSpecialChars,
  keymap,
  lineNumbers,
  placeholder as cmPlaceholder,
  rectangularSelection,
} from "@codemirror/view";
import {
  languageForFile,
  languageHasCompletion,
  languageNameForFile,
} from "@/utils/workspace/editorLanguages";
import { documentWordCompletion, hitFileCompletion } from "./editorCompletion";
import { hittableTheme } from "./editorTheme";

/**
 * Marks doc changes pushed in from the parent, so they are not echoed back out
 * through `onChange` — which would save the file the instant it finished loading.
 */
const ExternalSync = Annotation.define<boolean>();

type Props = {
  path: string[];
  value: string;
  onChange: (value: string) => void;
  onSave: () => void;
  disabled?: boolean;
};

export default function SourceEditor({ path, value, onChange, onSave, disabled }: Props) {
  const [wrap, setWrap] = useState(false);
  const [allFolded, setAllFolded] = useState(false);
  const [cursor, setCursor] = useState({ line: 1, column: 1, lines: 1 });

  const hostRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const wrapCompartment = useRef(new Compartment());
  const languageCompartment = useRef(new Compartment());
  const editableCompartment = useRef(new Compartment());

  // Callbacks live in refs so the editor is constructed once per file rather
  // than torn down whenever the parent re-renders with a new closure.
  const onChangeRef = useRef(onChange);
  const onSaveRef = useRef(onSave);
  onChangeRef.current = onChange;
  onSaveRef.current = onSave;

  const fileName = path.at(-1) ?? "Untitled";
  const languageName = languageNameForFile(fileName);

  const toggleWrap = useCallback(() => {
    setWrap((current) => {
      const next = !current;
      viewRef.current?.dispatch({
        effects: wrapCompartment.current.reconfigure(next ? EditorView.lineWrapping : []),
      });
      return next;
    });
  }, []);

  const toggleFoldAll = useCallback(() => {
    const view = viewRef.current;
    if (!view) return;
    setAllFolded((current) => {
      if (current) unfoldAll(view);
      else foldAll(view);
      view.focus();
      return !current;
    });
  }, []);

  // Build the editor once per file. `fileName` is in the dependency list so a
  // different file gets a fresh history and fold state.
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    const saveKeymap = Prec.high(
      keymap.of([
        {
          key: "Mod-s",
          preventDefault: true,
          run: () => {
            onSaveRef.current();
            return true;
          },
        },
        {
          key: "Alt-z",
          preventDefault: true,
          run: () => {
            toggleWrap();
            return true;
          },
        },
      ])
    );

    const view = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: value,
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightActiveLine(),
          highlightSpecialChars(),
          drawSelection(),
          rectangularSelection(),
          history(),
          indentOnInput(),
          indentUnit.of("  "),
          EditorState.tabSize.of(2),
          bracketMatching(),
          closeBrackets(),

          // Scope folding: arrows in the gutter, plus Ctrl-Shift-[ / ].
          codeFolding({ placeholderText: "⋯" }),
          foldGutter({ openText: "▾", closedText: "▸" }),

          autocompletion({ activateOnTyping: true, closeOnBlur: true, maxRenderedOptions: 60 }),
          // Sources beyond whatever the active grammar contributes: the `.hit`
          // schema, and buffer words as a floor for grammars that have no
          // completion of their own.
          EditorState.languageData.of(() => {
            if (fileName.endsWith(".hit")) return [{ autocomplete: hitFileCompletion }];
            if (languageHasCompletion(fileName)) return [];
            return [{ autocomplete: documentWordCompletion }];
          }),

          search({ top: true }),
          highlightSelectionMatches(),

          saveKeymap,
          keymap.of([
            ...closeBracketsKeymap,
            ...defaultKeymap,
            ...historyKeymap,
            ...foldKeymap,
            ...completionKeymap,
            ...searchKeymap,
            indentWithTab,
          ]),

          hittableTheme,
          wrapCompartment.current.of(wrap ? EditorView.lineWrapping : []),
          languageCompartment.current.of([]),
          editableCompartment.current.of([
            EditorView.editable.of(!disabled),
            EditorState.readOnly.of(!!disabled),
          ]),
          cmPlaceholder(disabled ? "Loading file…" : "Start typing…"),
          EditorView.contentAttributes.of({ "aria-label": `Edit ${fileName}` }),

          EditorView.updateListener.of((update) => {
            const external = update.transactions.some((tr) => tr.annotation(ExternalSync));
            if (update.docChanged && !external) {
              onChangeRef.current(update.state.doc.toString());
            }
            if (update.docChanged || update.selectionSet) {
              const head = update.state.selection.main.head;
              const line = update.state.doc.lineAt(head);
              setCursor({
                line: line.number,
                column: head - line.from + 1,
                lines: update.state.doc.lines,
              });
            }
          }),
        ],
      }),
    });

    viewRef.current = view;
    setCursor({ line: 1, column: 1, lines: view.state.doc.lines });

    // Grammars are code-split, so the file opens immediately and gains colour
    // a tick later. A stale load is ignored if the file changed meanwhile.
    let cancelled = false;
    const description = languageForFile(fileName);
    if (description) {
      void description.load().then((support) => {
        if (cancelled) return;
        view.dispatch({ effects: languageCompartment.current.reconfigure(support) });
      });
    }

    return () => {
      cancelled = true;
      view.destroy();
      viewRef.current = null;
    };
    // `value`, `wrap` and `disabled` are seeded here and then kept in sync by
    // the effects below, so re-running on their account would discard history.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fileName]);

  // Adopt content that changed outside the editor (a file finishing its read).
  useEffect(() => {
    const view = viewRef.current;
    if (!view || value === view.state.doc.toString()) return;
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: value },
      annotations: ExternalSync.of(true),
    });
  }, [value]);

  useEffect(() => {
    viewRef.current?.dispatch({
      effects: editableCompartment.current.reconfigure([
        EditorView.editable.of(!disabled),
        EditorState.readOnly.of(!!disabled),
      ]),
    });
  }, [disabled]);

  return (
    <div className="source-editor">
      <div className="source-toolbar">
        <div className="flex min-w-0 items-center gap-2" title={path.join("/")}>
          <FileCode2 size={16} className="shrink-0 text-cyan-300" aria-hidden="true" />
          <span className="truncate text-sm font-medium text-slate-100">{fileName}</span>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            onClick={toggleFoldAll}
            aria-pressed={allFolded}
            title={allFolded ? "Expand all scopes" : "Collapse all scopes"}
            className="workspace-button"
          >
            {allFolded ? (
              <ChevronsUpDown size={15} aria-hidden="true" />
            ) : (
              <ChevronsDownUp size={15} aria-hidden="true" />
            )}
            <span>{allFolded ? "Expand" : "Collapse"}</span>
          </button>
          <button
            type="button"
            onClick={toggleWrap}
            aria-pressed={wrap}
            title="Toggle word wrap (Alt+Z / Option+Z on Mac)"
            className="workspace-button"
          >
            <WrapText size={15} aria-hidden="true" />
            <span>Wrap</span>
          </button>
        </div>
      </div>
      {path.length > 1 && (
        <div className="source-breadcrumb" title={path.join(" / ")}>
          {path.join(" / ")}
        </div>
      )}
      <div ref={hostRef} className="source-content" />
      <div className="source-status">
        <span>
          Ln {cursor.line}, Col {cursor.column}
        </span>
        <span>
          {languageName} · {cursor.lines} {cursor.lines === 1 ? "line" : "lines"}
          <span className="hidden sm:inline"> · UTF-8</span>
        </span>
      </div>
    </div>
  );
}
