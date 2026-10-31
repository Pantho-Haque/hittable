import pako from "pako";
import type { THittableCurlJson } from "@/types";
import { HITTABLE_METHODS } from "@/constants/hittable";
import { compressString } from "@/utils/compressString";

// FR-15: a request travels in the URL fragment, which browsers never send to
// a server. The decoder treats the payload as hostile: it never throws, caps
// every size, and accepts only the exact shape below.

export const SHARE_MAX_BYTES = 32 * 1024;
const MAX_INFLATED = 256 * 1024;
const MAX = { name: 200, url: 8192, key: 256, value: 8192, entries: 100, body: 128 * 1024 };

type StrMap = Record<string, string>;
export type EncodeResult = { ok: true; fragment: string; redacted: string[] } | { ok: false; error: string };
export type DecodeResult = { ok: true; request: THittableCurlJson; name: string } | { ok: false; error: string };

const CREDENTIAL_HEADERS = new Set([
  "authorization", "proxy-authorization", "cookie", "x-api-key", "api-key", "x-auth-token", "x-access-token",
]);
const ONLY_TEMPLATES = /^\s*(<<[^<>]+>>\s*)+$/;

const isCredentialHeader = (name: string) => {
  const n = name.toLowerCase();
  return CREDENTIAL_HEADERS.has(n) || n.includes("secret") || n.includes("token");
};

/** Swap a literal credential for a `<<KEY>>` placeholder; null when nothing needs hiding. */
export function redactHeader(name: string, value: string): string | null {
  if (!isCredentialHeader(name) || ONLY_TEMPLATES.test(value)) return null;
  const upper = name.toUpperCase().replace(/[^A-Z0-9]+/g, "_").replace(/^_|_$/g, "");
  if (/authorization$/i.test(name)) {
    const m = value.trim().match(/^(\S+)\s+([\s\S]+)$/);
    if (m) {
      if (ONLY_TEMPLATES.test(m[2])) return null;
      const scheme = m[1].toLowerCase();
      const key = scheme === "bearer" ? "TOKEN" : scheme === "basic" ? "BASIC_AUTH" : upper;
      return `${m[1]} <<${key}>>`;
    }
  }
  return `<<${upper}>>`;
}

/** Parse an editor's JSON into a flat string map; "" means none. */
function flatMap(raw: string, label: string): StrMap | string {
  if (!raw.trim()) return {};
  const error = `${label} aren't a valid JSON object, so the link can't be made. Fix them and try again.`;
  let obj: unknown;
  try {
    obj = JSON.parse(raw);
  } catch {
    return error;
  }
  if (!obj || typeof obj !== "object" || Array.isArray(obj)) return error;
  const out: [string, string][] = [];
  for (const [k, v] of Object.entries(obj)) {
    if (v !== null && typeof v === "object") return error;
    out.push([k, String(v ?? "")]);
  }
  return Object.fromEntries(out);
}

export function encodeShare(req: THittableCurlJson, name = ""): EncodeResult {
  const headers = flatMap(req.headers, "Headers");
  if (typeof headers === "string") return { ok: false, error: headers };
  const params = flatMap(req.params, "Params");
  if (typeof params === "string") return { ok: false, error: params };

  const redacted: string[] = [];
  for (const [k, v] of Object.entries(headers)) {
    const safe = redactHeader(k, v);
    if (safe !== null) {
      headers[k] = safe;
      redacted.push(`${k} was replaced with ${safe}`);
    }
  }
  let url = req.url.replace(/^([a-z][a-z0-9+.-]*:\/\/)[^/?#@\s]*@/i, "$1");
  if (url !== req.url) redacted.push("The user name and password in the URL were removed");

  // Query parameters that carry credentials (?api_key=, ?access_token=…) are
  // shared as variables, in Params and in the URL, which mirror each other.
  const swapped = new Set<string>();
  for (const [k, v] of Object.entries(params)) {
    if (isSecretParam(k) && !onlyTemplates(v)) {
      params[k] = `<<${upperSnake(k)}>>`;
      swapped.add(k);
    }
  }
  url = url.replace(/([?&])([^=&#]+)=([^&#]*)/g, (m, sep: string, k: string, v: string) => {
    const key = safeDecode(k);
    if (!isSecretParam(key) || onlyTemplates(safeDecode(v))) return m;
    swapped.add(key);
    return `${sep}${k}=<<${upperSnake(key)}>>`;
  });
  for (const k of swapped) redacted.push(`The ${k} query parameter was replaced with <<${upperSnake(k)}>>`);

  const payload = { v: 1, name, method: req.method, url, headers, params, body: req.body ?? "" };
  const fragment = `share=${compressString(JSON.stringify(payload))}`;
  if (fragment.length > SHARE_MAX_BYTES) {
    return { ok: false, error: `This request is too large to share as a link (${Math.ceil(fragment.length / 1024)} KB, limit 32 KB).` };
  }
  return { ok: true, fragment, redacted };
}

const DAMAGED = "This share link is damaged or too large.";

function inflateCapped(bytes: Uint8Array): string | null {
  const inflator = new pako.Inflate();
  const chunks: Uint8Array[] = [];
  let total = 0;
  let ended = false; // pako calls onEnd only when the stream is complete
  inflator.onEnd = (status) => {
    ended = status === 0;
  };
  inflator.onData = (data) => {
    const chunk = data as Uint8Array;
    total += chunk.length;
    if (total > MAX_INFLATED) throw new Error("too large"); // a deflate bomb
    chunks.push(chunk);
  };
  inflator.push(bytes, true);
  if (inflator.err || !ended) return null; // corrupt or truncated
  const all = new Uint8Array(total);
  let at = 0;
  for (const c of chunks) {
    all.set(c, at);
    at += c.length;
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(all);
}

const str = (v: unknown, max: number): v is string => typeof v === "string" && v.length <= max;

function strictMap(v: unknown): StrMap | null {
  if (!v || typeof v !== "object" || Array.isArray(v)) return null;
  const entries = Object.entries(v);
  if (entries.length > MAX.entries) return null;
  for (const [k, val] of entries) if (!str(k, MAX.key) || !k || !str(val, MAX.value)) return null;
  return Object.fromEntries(entries) as StrMap;
}

export function decodeShare(fragment: string): DecodeResult {
  try {
    const data = fragment.replace(/^#?share=/, "");
    if (!data || data.length > SHARE_MAX_BYTES || !/^[A-Za-z0-9_-]+$/.test(data)) return { ok: false, error: DAMAGED };
    const bytes = Uint8Array.from(atob(data.replace(/-/g, "+").replace(/_/g, "/")), (c) => c.charCodeAt(0));
    const text = inflateCapped(bytes);
    if (text === null) return { ok: false, error: DAMAGED };
    const p = JSON.parse(text);
    if (!p || typeof p !== "object" || p.v !== 1) return { ok: false, error: DAMAGED };
    const headers = strictMap(p.headers);
    const params = strictMap(p.params);
    if (
      !str(p.name, MAX.name) ||
      !HITTABLE_METHODS.includes(p.method) ||
      !str(p.url, MAX.url) ||
      !str(p.body, MAX.body) ||
      !headers ||
      !params
    ) {
      return { ok: false, error: DAMAGED };
    }
    return {
      ok: true,
      name: p.name,
      request: {
        method: p.method,
        url: p.url,
        headers: JSON.stringify(headers, null, 2),
        params: JSON.stringify(params, null, 2),
        body: p.body,
      },
    };
  } catch {
    return { ok: false, error: DAMAGED };
  }
}

export const shareUrl = (origin: string, fragment: string) => `${origin}/hittable#${fragment}`;

const SECRET_PARAM = /(^|[_-])(api[_-]?key|apikey|key|token|access[_-]?token|refresh[_-]?token|id[_-]?token|secret|client[_-]?secret|password|passwd|pwd|sig|signature|auth)$/i;

function isSecretParam(name: string): boolean {
  return SECRET_PARAM.test(name);
}

function onlyTemplates(v: string): boolean {
  return /^(\s*<<\w+>>\s*)+$/.test(v);
}

function upperSnake(name: string): string {
  return name.replace(/([a-z0-9])([A-Z])/g, "$1_$2").replace(/[^A-Za-z0-9]+/g, "_").replace(/^_|_$/g, "").toUpperCase() || "VALUE";
}

function safeDecode(s: string): string {
  try {
    return decodeURIComponent(s.replace(/\+/g, " "));
  } catch {
    return s;
  }
}
