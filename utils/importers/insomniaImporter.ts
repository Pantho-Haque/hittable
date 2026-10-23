import { THittableCollection, THittableItem } from "@/types";
import { base64Utf8, jsonToCurl } from "@/utils/curlConverter";
import { addMissingVars, fromInsomnia } from "./variableSyntax";

// Mirrors the terminal app's importer (shellapp/internal/collection): each
// workspace becomes a collection, request groups become folders nested by
// parentId, and requests keep their headers, parameters, auth and body.

type NV = { name?: string; value?: unknown; disabled?: boolean; fileName?: string };

interface InsomniaResource {
  _type?: string;
  _id?: string;
  parentId?: string | null;
  name?: string;
  url?: string;
  method?: string;
  headers?: NV[] | Record<string, unknown>;
  parameters?: NV[];
  body?: { mimeType?: string; text?: string; params?: NV[] };
  authentication?: { type?: string; token?: string; prefix?: string; username?: string; password?: string; key?: string; value?: string; disabled?: boolean };
  data?: Record<string, unknown>;
  _kvPairData?: { name?: string; value?: unknown; enabled?: boolean }[];
}

const isObject = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
const list = <T,>(v: unknown): T[] => (Array.isArray(v) ? v.filter(isObject) as T[] : []);
const str = (v: unknown) => (v === undefined || v === null ? "" : String(v));

function resourcesOf(json: unknown): InsomniaResource[] {
  if (Array.isArray(json)) return list(json);
  return isObject(json) ? list(json.resources) : [];
}

function buildCurl(req: InsomniaResource): string {
  const method = str(req.method || "GET").toUpperCase();
  const headers: Record<string, string> = {};

  const a = req.authentication;
  if (isObject(a) && !a.disabled) {
    if (a.type === "bearer") headers.Authorization = `${a.prefix || "Bearer"} ${fromInsomnia(str(a.token))}`;
    else if (a.type === "basic") {
      const creds = `${fromInsomnia(str(a.username))}:${fromInsomnia(str(a.password))}`;
      headers.Authorization = /<<\w+>>/.test(creds)
        ? `Basic <<BASIC_${fromInsomnia(str(a.username)).replace(/\W/g, "_")}>>`
        : `Basic ${base64Utf8(creds)}`;
    } else if (a.type === "apikey" && a.key) headers[fromInsomnia(a.key)] = fromInsomnia(str(a.value));
  }

  // Headers: an array of {name, value, disabled} (v4) or a plain object.
  if (Array.isArray(req.headers)) {
    for (const h of list<NV>(req.headers)) if (h.name && !h.disabled) headers[h.name] = fromInsomnia(str(h.value));
  } else if (isObject(req.headers)) {
    for (const [k, v] of Object.entries(req.headers)) headers[k] = fromInsomnia(str(v));
  }

  let url = fromInsomnia(str(req.url));
  const params = list<NV>(req.parameters).filter((p) => p.name && !p.disabled);
  if (params.length) {
    const enc = (v: unknown) => encodeURIComponent(fromInsomnia(str(v))).replace(/%3C%3C(\w+)%3E%3E/g, "<<$1>>");
    url += (url.includes("?") ? "&" : "?") + params.map((p) => `${enc(p.name)}=${enc(p.value)}`).join("&");
  }

  let body = "";
  const b = isObject(req.body) ? req.body : undefined;
  if (b?.text) {
    body = fromInsomnia(str(b.text));
    if (b.mimeType) headers["Content-Type"] ??= b.mimeType;
  } else if (b && list<NV>(b.params).length) {
    const obj: Record<string, string> = {};
    for (const p of list<NV>(b.params)) {
      if (p.name && !p.disabled) obj[p.name] = p.fileName ? `@${p.fileName}` : fromInsomnia(str(p.value));
    }
    body = JSON.stringify(obj, null, "\t");
    if (b.mimeType) headers["Content-Type"] ??= b.mimeType;
  }

  return jsonToCurl({ method, url, headers: JSON.stringify(headers), body, params: "{}" });
}

function extractEnvironmentVars(resources: InsomniaResource[]): Record<string, string> {
  const env: Record<string, string> = {};
  for (const r of resources.filter((r) => r._type === "environment")) {
    for (const pair of list<{ name?: string; value?: unknown; enabled?: boolean }>(r._kvPairData)) {
      if (pair.enabled !== false && pair.name) env[pair.name] = str(pair.value);
    }
    if (isObject(r.data)) {
      for (const [k, v] of Object.entries(r.data)) {
        if (k && (typeof v === "string" || typeof v === "number" || typeof v === "boolean")) env[k] = String(v);
      }
    }
  }
  return env;
}

export function parseInsomniaExport(json: unknown): THittableCollection[] {
  const resources = resourcesOf(json);
  const byId = new Map<string, InsomniaResource>();
  for (const r of resources) if (r._id) byId.set(r._id, r);

  const childrenOf = (parentId: string, seen: Set<string>): THittableItem[] => {
    const items: THittableItem[] = [];
    for (const r of resources) {
      if (str(r.parentId) !== parentId) continue;
      if (r._type === "request_group" && r._id && !seen.has(r._id)) {
        seen.add(r._id);
        items.push({ type: "folder", name: r.name || "Unnamed Folder", items: childrenOf(r._id, seen) });
      } else if (r._type === "request") {
        items.push({ type: "route", name: r.name || "Unnamed", curl: buildCurl(r), response: "" });
      }
    }
    return items;
  };

  const env = extractEnvironmentVars(resources);
  // Each workspace is a collection. Groups and requests whose parent is not in
  // the export (a bare resource list, a placeholder id) form one more.
  const roots: [string[], string][] = resources
    .filter((r) => r._type === "workspace" && r._id)
    .map((w) => [[str(w._id)], w.name || "Imported Collection"]);
  const orphanParents = [...new Set(resources
    .filter((r) => (r._type === "request" || r._type === "request_group") && !byId.has(str(r.parentId)))
    .map((r) => str(r.parentId)))];
  if (orphanParents.length) roots.push([orphanParents, roots[0]?.[1] ?? "Imported Collection"]);

  const collections: THittableCollection[] = [];
  const seen = new Set<string>();
  for (const [parentIds, name] of roots) {
    const items = parentIds.flatMap((id) => childrenOf(id, seen));
    if (!items.length) continue;
    const colEnv = { ...env };
    const scan = (i: THittableItem): void => (i.type === "route" ? addMissingVars(i.curl, colEnv) : i.items.forEach(scan));
    items.forEach(scan);
    collections.push({ collectionName: name, items, env: colEnv, secrets: {} });
  }
  return collections;
}

export function isInsomniaExport(json: unknown): boolean {
  if (Array.isArray(json)) return json.length > 0 && isObject(json[0]) && typeof json[0]._type === "string";
  if (!isObject(json)) return false;
  if (json._type === "export" || json.__export_format === 4) return true;
  const first = Array.isArray(json.resources) ? json.resources[0] : undefined;
  return isObject(first) && typeof first._type === "string";
}
