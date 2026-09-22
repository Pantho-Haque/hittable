import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorView } from "@codemirror/view";
import { tags as t } from "@lezer/highlight";

/**
 * Colours are kept in sync with styles/workspace.css so the CodeMirror surface
 * is indistinguishable from the panes around it.
 */
const BACKGROUND = "#0c1422";
const FOREGROUND = "#dce6f3";
const GUTTER_BACKGROUND = "#0a1220";
const GUTTER_FOREGROUND = "#8191a8";
const CARET = "#67e8f9";
const SELECTION = "#164e63";
const BORDER = "#ffffff0f";
const MUTED = "#a8b5c7";

export const hittableEditorTheme = EditorView.theme(
  {
    "&": {
      color: FOREGROUND,
      backgroundColor: BACKGROUND,
      height: "100%",
      fontSize: "0.875rem",
    },
    ".cm-scroller": {
      fontFamily: 'var(--font-geist-mono), "SFMono-Regular", Consolas, monospace',
      lineHeight: "1.625rem",
      fontVariantLigatures: "none",
      overflow: "auto",
    },
    ".cm-content": {
      caretColor: CARET,
      padding: "1rem 0",
    },
    ".cm-line": { padding: "0 1rem" },

    "&.cm-focused": { outline: "none" },
    ".cm-cursor, .cm-dropCursor": { borderLeftColor: CARET, borderLeftWidth: "2px" },
    "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection": {
      backgroundColor: SELECTION,
    },
    ".cm-selectionMatch": { backgroundColor: "#1e3a5f" },
    "&.cm-focused .cm-matchingBracket": {
      backgroundColor: "#164e6366",
      outline: `1px solid ${CARET}55`,
    },
    "&.cm-focused .cm-nonmatchingBracket": { outline: "1px solid #f8717188" },
    ".cm-activeLine": { backgroundColor: "#ffffff06" },
    ".cm-activeLineGutter": { backgroundColor: "#ffffff0a", color: FOREGROUND },

    ".cm-gutters": {
      backgroundColor: GUTTER_BACKGROUND,
      color: GUTTER_FOREGROUND,
      border: "none",
      borderRight: `1px solid ${BORDER}`,
      userSelect: "none",
    },
    ".cm-lineNumbers .cm-gutterElement": { padding: "0 0.75rem 0 1rem", minWidth: "3ch" },

    // The fold arrows are the scope toggles. They stay faintly visible so the
    // foldable lines are discoverable, and sharpen on hover.
    ".cm-foldGutter": { paddingInline: "0.125rem" },
    ".cm-foldGutter .cm-gutterElement": {
      color: "#ffffff38",
      cursor: "pointer",
      transition: "color 120ms ease",
    },
    ".cm-gutters:hover .cm-foldGutter .cm-gutterElement": { color: GUTTER_FOREGROUND },
    ".cm-foldGutter .cm-gutterElement:hover": { color: CARET },
    ".cm-foldPlaceholder": {
      backgroundColor: "#ffffff0d",
      border: `1px solid ${BORDER}`,
      borderRadius: "4px",
      color: MUTED,
      margin: "0 0.25rem",
      padding: "0 0.375rem",
    },

    ".cm-panels": { backgroundColor: "#101c2d", color: FOREGROUND },
    ".cm-panels.cm-panels-bottom": { borderTop: `1px solid ${BORDER}` },
    ".cm-searchMatch": { backgroundColor: "#ca8a0455", outline: "1px solid #ca8a0499" },
    ".cm-searchMatch.cm-searchMatch-selected": { backgroundColor: "#ca8a0499" },
    ".cm-panel input, .cm-panel button": {
      backgroundColor: "#0e1f35",
      border: `1px solid ${BORDER}`,
      borderRadius: "4px",
      color: FOREGROUND,
      padding: "0.125rem 0.375rem",
    },

    ".cm-tooltip": {
      backgroundColor: "#101c2d",
      border: `1px solid ${BORDER}`,
      borderRadius: "6px",
      boxShadow: "0 12px 32px rgba(0, 0, 0, 0.55)",
      color: FOREGROUND,
      overflow: "hidden",
    },
    ".cm-tooltip.cm-tooltip-autocomplete > ul": {
      fontFamily: 'var(--font-geist-mono), "SFMono-Regular", Consolas, monospace',
      fontSize: "0.8125rem",
      maxHeight: "16rem",
    },
    ".cm-tooltip.cm-tooltip-autocomplete > ul > li": { padding: "0.1875rem 0.625rem" },
    ".cm-tooltip-autocomplete > ul > li[aria-selected]": {
      backgroundColor: "#164e63",
      color: "#f8fafc",
    },
    ".cm-completionLabel": { color: FOREGROUND },
    ".cm-completionMatchedText": { color: CARET, textDecoration: "none", fontWeight: "600" },
    ".cm-completionDetail": { color: MUTED, fontStyle: "normal", marginLeft: "0.75rem" },
    ".cm-completionIcon": { color: MUTED, opacity: 0.8, paddingRight: "0.625rem" },
  },
  { dark: true }
);

export const hittableHighlightStyle = HighlightStyle.define([
  { tag: [t.comment, t.lineComment, t.blockComment, t.docComment], color: "#5d7290", fontStyle: "italic" },

  { tag: [t.keyword, t.modifier, t.controlKeyword, t.moduleKeyword], color: "#c084fc" },
  { tag: [t.operator, t.operatorKeyword, t.punctuation, t.separator], color: "#94a3b8" },
  { tag: [t.bracket, t.paren, t.squareBracket, t.brace], color: "#cbd5e1" },

  { tag: [t.string, t.special(t.string), t.regexp], color: "#86efac" },
  { tag: [t.escape], color: "#5eead4" },
  { tag: [t.number, t.bool, t.null, t.atom, t.integer, t.float], color: "#fbbf24" },

  { tag: [t.function(t.variableName), t.function(t.propertyName), t.macroName], color: "#67e8f9" },
  { tag: [t.definition(t.variableName), t.definition(t.propertyName)], color: "#e2e8f0" },
  { tag: [t.variableName, t.labelName], color: FOREGROUND },
  { tag: [t.propertyName, t.attributeName], color: "#7dd3fc" },
  { tag: [t.className, t.definition(t.className), t.namespace], color: "#fcd34d" },
  { tag: [t.typeName, t.standard(t.typeName), t.annotation], color: "#5eead4" },
  { tag: [t.self, t.constant(t.variableName), t.standard(t.variableName)], color: "#f472b6" },

  { tag: [t.tagName, t.angleBracket], color: "#f472b6" },
  { tag: [t.attributeValue], color: "#86efac" },

  { tag: [t.heading], color: "#67e8f9", fontWeight: "700" },
  { tag: [t.heading1, t.heading2], color: "#67e8f9", fontWeight: "700" },
  { tag: [t.link, t.url], color: "#7dd3fc", textDecoration: "underline" },
  { tag: [t.emphasis], fontStyle: "italic" },
  { tag: [t.strong], fontWeight: "700" },
  { tag: [t.strikethrough], textDecoration: "line-through" },
  { tag: [t.monospace], color: "#fbbf24" },
  { tag: [t.list, t.quote], color: "#a8b5c7" },

  { tag: [t.inserted], color: "#86efac" },
  { tag: [t.deleted], color: "#fca5a5" },
  { tag: [t.changed], color: "#fbbf24" },
  { tag: [t.invalid], color: "#fca5a5", textDecoration: "underline wavy" },
]);

export const hittableTheme = [
  hittableEditorTheme,
  syntaxHighlighting(hittableHighlightStyle, { fallback: true }),
];
