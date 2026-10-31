// Structural diff of two JSON values, for comparing a response with the one
// before it. Object keys are compared as sets (order is not a change), arrays
// index by index.

import { formatPath, parseJsonPath, pathMatches, type PathStep, type Segment } from "./jsonPath";

export type DiffKind = "added" | "removed" | "changed";
export type DiffEntry = { path: string; kind: DiffKind; before?: unknown; after?: unknown };

const isObject = (v: unknown): v is Record<string, unknown> =>
  typeof v === "object" && v !== null && !Array.isArray(v);

/**
 * Every path where `b` differs from `a`. `ignore` takes JSONPath patterns
 * (`$.updatedAt`, `$.items[*].id`, `$..etag`); a matching path and
 * everything under it is left out. Patterns that do not parse are skipped.
 */
export function diffJson(a: unknown, b: unknown, opts: { ignore?: string[] } = {}): DiffEntry[] {
  const ignore: Segment[][] = [];
  for (const p of opts.ignore ?? []) {
    const parsed = parseJsonPath(p);
    if (parsed.ok) ignore.push(parsed.segments);
  }
  const out: DiffEntry[] = [];
  walk(a, b, [], ignore, out);
  return out;
}

function walk(a: unknown, b: unknown, steps: PathStep[], ignore: Segment[][], out: DiffEntry[]) {
  if (ignore.some((segs) => pathMatches(segs, steps))) return;
  const push = (kind: DiffKind, s: PathStep[], before?: unknown, after?: unknown) => {
    if (s !== steps && ignore.some((segs) => pathMatches(segs, s))) return;
    const e: DiffEntry = { path: formatPath(s), kind };
    if (kind !== "added") e.before = before;
    if (kind !== "removed") e.after = after;
    out.push(e);
  };

  if (Array.isArray(a) && Array.isArray(b)) {
    for (let i = 0; i < Math.max(a.length, b.length); i++) {
      if (i >= a.length) push("added", [...steps, i], undefined, b[i]);
      else if (i >= b.length) push("removed", [...steps, i], a[i]);
      else walk(a[i], b[i], [...steps, i], ignore, out);
    }
    return;
  }
  if (isObject(a) && isObject(b)) {
    const keys = [...new Set([...Object.keys(a), ...Object.keys(b)])].sort();
    for (const k of keys) {
      if (!Object.hasOwn(a, k)) push("added", [...steps, k], undefined, b[k]);
      else if (!Object.hasOwn(b, k)) push("removed", [...steps, k], a[k]);
      else walk(a[k], b[k], [...steps, k], ignore, out);
    }
    return;
  }
  if (a !== b) push("changed", steps, a, b);
}

export type HeaderDiff = { name: string; kind: DiffKind; before?: string; after?: string };

function lowerKeys(h: unknown): Map<string, string> {
  const m = new Map<string, string>();
  if (isObject(h)) for (const [k, v] of Object.entries(h)) m.set(k.toLowerCase(), String(v));
  return m;
}

/** Header changes by lower-cased name; `ignore` names are matched case-insensitively. */
export function diffHeaders(a: unknown, b: unknown, ignore: string[] = []): HeaderDiff[] {
  const before = lowerKeys(a);
  const after = lowerKeys(b);
  const skip = new Set(ignore.map((n) => n.toLowerCase()));
  const names = [...new Set([...before.keys(), ...after.keys()])].sort().filter((n) => !skip.has(n));
  return names.flatMap((name): HeaderDiff[] => {
    const x = before.get(name);
    const y = after.get(name);
    if (x === y) return [];
    if (x === undefined) return [{ name, kind: "added", after: y }];
    if (y === undefined) return [{ name, kind: "removed", before: x }];
    return [{ name, kind: "changed", before: x, after: y }];
  });
}
