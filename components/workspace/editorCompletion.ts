import type { Completion, CompletionContext, CompletionResult } from "@codemirror/autocomplete";
import { syntaxTree } from "@codemirror/language";
import { EditorState, type Text } from "@codemirror/state";

const WORD = /[\w$]+/g;
const MIN_WORD_LENGTH = 3;
const MAX_WORDS = 2000;
/** Past this size, scanning the whole document on every keystroke stops being free. */
const MAX_SCANNED_BYTES = 1_000_000;

/**
 * Text instances are immutable and replaced on every edit, so a WeakMap keyed
 * by the document both caches the scan and evicts itself as the user types.
 */
const wordCache = new WeakMap<Text, string[]>();

function documentWords(doc: Text): string[] {
  const cached = wordCache.get(doc);
  if (cached) return cached;

  const words: string[] = [];
  if (doc.length <= MAX_SCANNED_BYTES) {
    const seen = new Set<string>();
    const text = doc.toString();
    for (const match of text.matchAll(WORD)) {
      const word = match[0];
      if (word.length < MIN_WORD_LENGTH || seen.has(word)) continue;
      seen.add(word);
      words.push(word);
      if (words.length >= MAX_WORDS) break;
    }
  }

  wordCache.set(doc, words);
  return words;
}

/**
 * A last-resort completer so every file type — including ones with no grammar
 * at all — still completes identifiers already present in the buffer.
 */
export function documentWordCompletion(context: CompletionContext): CompletionResult | null {
  const before = context.matchBefore(/[\w$]+/);
  if (!before || (before.from === before.to && !context.explicit)) return null;

  const typed = context.state.sliceDoc(before.from, before.to);
  const options: Completion[] = documentWords(context.state.doc)
    // The half-typed word under the cursor is not a useful suggestion.
    .filter((word) => word !== typed)
    .map((word) => ({ label: word, type: "text", boost: -50 }));

  if (options.length === 0) return null;
  return { from: before.from, options, validFor: /^[\w$]*$/ };
}

const HIT_KEYS = ["method", "url", "headers", "params", "body", "response"] as const;

const HTTP_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"];

const COMMON_HEADERS = [
  "Accept",
  "Accept-Encoding",
  "Accept-Language",
  "Authorization",
  "Cache-Control",
  "Connection",
  "Content-Length",
  "Content-Type",
  "Cookie",
  "Host",
  "If-Match",
  "If-None-Match",
  "Origin",
  "Referer",
  "User-Agent",
  "X-Api-Key",
  "X-Correlation-Id",
  "X-Request-Id",
];

const CONTENT_TYPES = [
  "application/json",
  "application/x-www-form-urlencoded",
  "application/xml",
  "multipart/form-data",
  "text/html",
  "text/plain",
];

/** Names of the JSON properties enclosing `pos`, outermost first. */
function propertyPath(state: EditorState, pos: number): string[] {
  const path: string[] = [];
  let node = syntaxTree(state).resolveInner(pos, -1);

  while (node.parent) {
    if (node.name === "Property") {
      const nameNode = node.getChild("PropertyName");
      // When the cursor sits in the property's own name the user is still
      // choosing that key, so the meaningful context is the enclosing object.
      const namingThisProperty = nameNode && pos >= nameNode.from && pos <= nameNode.to;
      if (nameNode && !namingThisProperty) {
        path.unshift(state.sliceDoc(nameNode.from, nameNode.to).replace(/^"|"$/g, ""));
      }
    }
    node = node.parent;
  }

  return path;
}

function stringOptions(values: readonly string[], type: string): Completion[] {
  return values.map((value) => ({ label: value, type }));
}

/**
 * Hittable's own `.hit` request files are JSON with a fixed shape, so the
 * schema is worth completing directly: keys at the top level, verbs after
 * `"method"`, and real header names inside `"headers"`.
 */
export function hitFileCompletion(context: CompletionContext): CompletionResult | null {
  // Completing inside a string means the quote is already open; otherwise the
  // accepted completion has to bring its own quotes.
  const inString = context.matchBefore(/"[^"]*/);
  const before = inString ?? context.matchBefore(/[\w-]*/);
  if (!before || (before.from === before.to && !context.explicit)) return null;

  // `matchBefore` includes the opening quote, which must survive the insert.
  const from = inString ? before.from + 1 : before.from;

  const path = propertyPath(context.state, context.pos);
  const quote = (options: Completion[]): Completion[] =>
    inString ? options : options.map((o) => ({ ...o, apply: `"${o.label}"` }));

  const last = path[path.length - 1];

  if (last === "method") {
    return { from, options: quote(stringOptions(HTTP_METHODS, "enum")) };
  }

  if (last?.toLowerCase() === "content-type") {
    return { from, options: quote(stringOptions(CONTENT_TYPES, "enum")) };
  }

  if (last === "headers") {
    return { from, options: quote(stringOptions(COMMON_HEADERS, "property")) };
  }

  if (path.length === 0) {
    const options = HIT_KEYS.map<Completion>((key) => ({
      label: key,
      type: "property",
      apply: inString ? key : `"${key}": `,
    }));
    return { from, options };
  }

  return null;
}
