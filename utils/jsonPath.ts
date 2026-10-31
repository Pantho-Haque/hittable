// A small JSONPath engine (RFC 9535 subset) for the response filter and the
// diff's ignore list. Pure: no eval, no filter expressions.
//
//   $            root
//   .key ['key'] ["key"]   member
//   [n] [-n]     index (negative counts from the end)
//   [*] .*       every child
//   [start:end]  array slice (either side optional, negatives from the end)
//   ..sel        recursive descent: sel applied to every node at any depth

export type Selector =
  | { type: "name"; name: string }
  | { type: "index"; index: number }
  | { type: "wild" }
  | { type: "slice"; start?: number; end?: number };

export type Segment = Selector & { descendant: boolean };

/** A concrete step in a normalized path: an object key or an array index. */
export type PathStep = string | number;

export type ParseResult = { ok: true; segments: Segment[] } | { ok: false; error: string };
export type QueryResult = { ok: true; matches: unknown[]; paths: string[] } | { ok: false; error: string };

const INT = /^-?\d+$/;

/** Parse a path into segments, or say what is wrong with it. */
export function parseJsonPath(input: string): ParseResult {
  const s = input.trim();
  if (!s.startsWith("$")) return { ok: false, error: "a path starts with $" };
  const segments: Segment[] = [];
  let i = 1;
  const fail = (error: string): ParseResult => ({ ok: false, error });

  while (i < s.length) {
    let descendant = false;
    if (s.startsWith("..", i)) {
      descendant = true;
      i += 2;
      if (i >= s.length) return fail("`..` needs a name, * or [ ] after it");
    } else if (s[i] === ".") {
      i += 1;
      if (i >= s.length) return fail("`.` needs a name or * after it");
      if (s[i] === "[" || s[i] === ".") return fail(`unexpected "${s[i]}" after "."`);
    } else if (s[i] !== "[") {
      return fail(`unexpected "${s[i]}" at position ${i}`);
    }

    if (s[i] === "[") {
      const parsed = parseBracket(s, i);
      if ("error" in parsed) return fail(parsed.error);
      segments.push({ ...parsed.selector, descendant });
      i = parsed.next;
      continue;
    }
    if (s[i] === "*") {
      segments.push({ type: "wild", descendant });
      i += 1;
      continue;
    }
    let j = i;
    while (j < s.length && !".[]*'\" \t\n".includes(s[j])) j++;
    if (j === i) return fail(`unexpected "${s[i]}" at position ${i}`);
    segments.push({ type: "name", name: s.slice(i, j), descendant });
    i = j;
  }
  return { ok: true, segments };
}

function parseBracket(s: string, start: number): { selector: Selector; next: number } | { error: string } {
  let i = start + 1;
  const quote = s[i];
  if (quote === "'" || quote === '"') {
    let name = "";
    i++;
    while (i < s.length && s[i] !== quote) {
      if (s[i] === "\\") {
        i++;
        if (i >= s.length) break;
      }
      name += s[i++];
    }
    if (i >= s.length) return { error: "unterminated string" };
    i++; // closing quote
    if (s[i] !== "]") return { error: "expected ] after the quoted name" };
    return { selector: { type: "name", name }, next: i + 1 };
  }
  const close = s.indexOf("]", i);
  if (close === -1) return { error: "missing ]" };
  const body = s.slice(i, close).trim();
  const next = close + 1;
  if (body === "*") return { selector: { type: "wild" }, next };
  if (INT.test(body)) return { selector: { type: "index", index: Number(body) }, next };
  const parts = body.split(":");
  if (parts.length === 2 && parts.every((p) => p.trim() === "" || INT.test(p.trim()))) {
    const [a, b] = parts.map((p) => (p.trim() === "" ? undefined : Number(p)));
    return { selector: { type: "slice", start: a, end: b }, next };
  }
  if (body.startsWith("?")) return { error: "filter expressions are not supported" };
  if (body === "") return { error: "empty [ ]" };
  return { error: `"${body}" is not an index, slice, * or quoted name` };
}

const isObject = (v: unknown): v is Record<string, unknown> =>
  typeof v === "object" && v !== null && !Array.isArray(v);

/** Children of `value` picked by one selector, with the step that reaches each. */
function select(value: unknown, sel: Selector): [PathStep, unknown][] {
  if (sel.type === "name") {
    return isObject(value) && Object.hasOwn(value, sel.name) ? [[sel.name, value[sel.name]]] : [];
  }
  if (sel.type === "wild") {
    if (Array.isArray(value)) return value.map((v, i) => [i, v]);
    return isObject(value) ? Object.entries(value) : [];
  }
  if (!Array.isArray(value)) return [];
  const len = value.length;
  if (sel.type === "index") {
    const i = sel.index < 0 ? len + sel.index : sel.index;
    return i >= 0 && i < len ? [[i, value[i]]] : [];
  }
  const clamp = (n: number) => Math.min(Math.max(n < 0 ? len + n : n, 0), len);
  const from = clamp(sel.start ?? 0);
  const to = clamp(sel.end ?? len);
  const out: [PathStep, unknown][] = [];
  for (let i = from; i < to; i++) out.push([i, value[i]]);
  return out;
}

type Node = { value: unknown; steps: PathStep[] };

function descendantsOrSelf(node: Node, out: Node[] = []): Node[] {
  out.push(node);
  for (const [step, child] of select(node.value, { type: "wild" })) {
    descendantsOrSelf({ value: child, steps: [...node.steps, step] }, out);
  }
  return out;
}

/** `$['a'][0]` — the normalized form of a concrete path. */
export function formatPath(steps: PathStep[]): string {
  return "$" + steps.map((s) => (typeof s === "number" ? `[${s}]` : `['${s.replace(/\\/g, "\\\\").replace(/'/g, "\\'")}']`)).join("");
}

export function queryJsonPath(data: unknown, path: string): QueryResult {
  const parsed = parseJsonPath(path);
  if (!parsed.ok) return parsed;
  let nodes: Node[] = [{ value: data, steps: [] }];
  for (const seg of parsed.segments) {
    const from = seg.descendant ? nodes.flatMap((n) => descendantsOrSelf(n)) : nodes;
    nodes = from.flatMap((n) => select(n.value, seg).map(([step, value]) => ({ value, steps: [...n.steps, step] })));
  }
  return { ok: true, matches: nodes.map((n) => n.value), paths: nodes.map((n) => formatPath(n.steps)) };
}

function stepMatches(sel: Selector, step: PathStep): boolean {
  switch (sel.type) {
    case "wild":
      return true;
    case "name":
      return step === sel.name;
    case "index":
      return step === sel.index;
    case "slice":
      return typeof step === "number" && step >= (sel.start ?? 0) && (sel.end === undefined || step < sel.end);
  }
}

/**
 * Whether a pattern matches `steps` or one of its ancestors, so ignoring
 * `$.meta` also ignores everything under it. Negative indexes and slice ends
 * only match by their literal value: the pattern has no array to count from.
 */
export function pathMatches(segments: Segment[], steps: PathStep[], si = 0, pi = 0): boolean {
  if (pi === segments.length) return true;
  const seg = segments[pi];
  if (!seg.descendant) {
    return si < steps.length && stepMatches(seg, steps[si]) && pathMatches(segments, steps, si + 1, pi + 1);
  }
  for (let k = si; k < steps.length; k++) {
    if (stepMatches(seg, steps[k]) && pathMatches(segments, steps, k + 1, pi + 1)) return true;
  }
  return false;
}
