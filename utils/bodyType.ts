// Request body types. The type is never stored: it is inferred from the body
// and its Content-Type header, so .hit files, local collections and curl
// exports stay exactly as they were (and the terminal app reads them as-is).

import { MULTIPART, parseFileRefs } from "./fileBody";

export type BodyType = "none" | "json" | "text" | "form" | "multipart" | "file";

export const BODY_TYPES: { id: BodyType; label: string; contentType?: string }[] = [
  { id: "none", label: "None" },
  { id: "json", label: "JSON", contentType: "application/json" },
  { id: "text", label: "Text", contentType: "text/plain" },
  { id: "form", label: "Form", contentType: "application/x-www-form-urlencoded" },
  { id: "multipart", label: "Multipart", contentType: MULTIPART },
  // A file body keeps the user's Content-Type, or is sent with one guessed from the file name.
  { id: "file", label: "File" },
];

// Content types this UI sets itself; only these are replaced when the user
// switches type. Anything else the user typed (e.g. application/vnd.api+json)
// is theirs and wins.
const AUTO_TYPES = BODY_TYPES.map((t) => t.contentType).filter(Boolean) as string[];

function parseHeaders(headers: string): Record<string, string> | null {
  try {
    const h = JSON.parse(headers || "{}");
    return h && typeof h === "object" && !Array.isArray(h) ? (h as Record<string, string>) : null;
  } catch {
    return null;
  }
}

function contentTypeKey(h: Record<string, string>): string | undefined {
  return Object.keys(h).find((k) => k.toLowerCase() === "content-type");
}

export function getContentType(headers: string): string {
  const h = parseHeaders(headers);
  const k = h && contentTypeKey(h);
  return k ? String(h![k]) : "";
}

function isFileBody(body: string): boolean {
  try {
    return parseFileRefs(body, "")?.kind === "binary";
  } catch {
    return true; // `@` with no path: a file body still being typed
  }
}

export function inferBodyType(body: string, headers: string): BodyType {
  const ct = getContentType(headers).toLowerCase();
  if (ct.includes("x-www-form-urlencoded")) return "form";
  if (ct.startsWith(MULTIPART)) return "multipart";
  // `@path` on one line is a file, whatever the content type (a JSON file can be the body).
  if (isFileBody(body)) return "file";
  if (/[/+]json\b/.test(ct)) return "json";
  if (ct.startsWith("text/")) return "text";
  const t = body.trim();
  // An empty body opens in the JSON editor, ready to type into. "None" is
  // only ever an explicit choice (see TabEditor), since it is not stored.
  if (!t) return ct ? "text" : "json";
  // Something that starts like JSON is JSON, even half-typed: it must be
  // flagged as invalid rather than quietly sent as text.
  if (t.startsWith("{") || t.startsWith("[")) return "json";
  try {
    JSON.parse(t);
    return "json";
  } catch {
    return "text";
  }
}

/**
 * Headers after switching to `type`: sets the matching Content-Type when
 * there is none or when the current one is one this UI set, and removes it
 * for "none". Returns the input unchanged if the headers are not valid JSON.
 */
export function headersForBodyType(headers: string, type: BodyType): string {
  const h = parseHeaders(headers);
  if (!h) return headers;
  const key = contentTypeKey(h);
  const current = key ? String(h[key]).toLowerCase() : "";
  const ours = !key || AUTO_TYPES.some((t) => current.startsWith(t));
  if (!ours) return headers;
  const next = { ...h };
  if (key) delete next[key];
  const ct = BODY_TYPES.find((t) => t.id === type)?.contentType;
  if (ct) next[key ?? "Content-Type"] = ct;
  return JSON.stringify(next, null, "\t");
}

export function formToEntries(body: string): [string, string][] {
  const entries = [...new URLSearchParams(body).entries()];
  return entries.length ? entries : [["", ""]];
}

export function entriesToForm(entries: [string, string][]): string {
  const p = new URLSearchParams();
  for (const [k, v] of entries) if (k.trim()) p.append(k, v);
  return p.toString();
}
