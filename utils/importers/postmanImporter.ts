import { THittableCollection, THittableItem } from "@/types";
import { base64Utf8, jsonToCurl } from "@/utils/curlConverter";
import { addMissingVars, fromPostman } from "./variableSyntax";

// Mirrors the terminal app's importer (shellapp/internal/collection): one
// Postman collection becomes one Hittable collection, item groups become
// folders, and bearer / basic / apikey auth becomes a header.

type KV = { key?: string; value?: unknown; disabled?: boolean; type?: string; src?: unknown; enabled?: boolean };

interface PostmanAuth {
  type?: string;
  bearer?: KV[];
  basic?: KV[];
  apikey?: KV[];
}

interface PostmanUrl {
  raw?: string;
  protocol?: string;
  host?: string | (string | { value?: string })[];
  port?: string;
  path?: string | (string | { value?: string })[];
  query?: KV[];
}

interface PostmanRequest {
  method?: string;
  url?: string | PostmanUrl;
  header?: KV[];
  body?: { mode?: string; raw?: string; urlencoded?: KV[]; formdata?: KV[] };
  auth?: PostmanAuth | null;
}

interface PostmanItem {
  name?: string;
  disabled?: boolean;
  item?: unknown;
  request?: PostmanRequest | string;
  variable?: unknown;
  auth?: PostmanAuth | null;
}

interface PostmanCollection {
  info?: { name?: string; schema?: string };
  item?: unknown;
  variable?: unknown;
  auth?: PostmanAuth | null;
}

const isObject = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
const list = <T,>(v: unknown): T[] => (Array.isArray(v) ? v.filter(isObject) as T[] : []);
const str = (v: unknown) => (v === undefined || v === null ? "" : String(v));

// The auth headers an auth block produces; null means "nothing to add".
function authHeaders(auth: PostmanAuth | null | undefined): Record<string, string> | null {
  if (!isObject(auth)) return null;
  const get = (kvs: unknown, key: string) => fromPostman(str(list<KV>(kvs).find((kv) => kv.key === key)?.value));
  switch (auth.type) {
    case "bearer":
      return { Authorization: `Bearer ${get(auth.bearer, "token")}` };
    case "basic": {
      const user = get(auth.basic, "username");
      const pass = get(auth.basic, "password");
      // Templated credentials cannot be encoded now; leave a variable to fill in.
      if (/<<\w+>>/.test(user + pass)) return { Authorization: `Basic <<BASIC_${user.replace(/\W/g, "_")}>>` };
      return { Authorization: `Basic ${base64Utf8(`${user}:${pass}`)}` };
    }
    case "apikey":
      if (get(auth.apikey, "in") === "query") return null; // like the terminal app: header keys only
      return { [get(auth.apikey, "key") || "X-API-Key"]: get(auth.apikey, "value") };
    default:
      return null; // noauth, inherit, and unsupported types (reported by the import dialog)
  }
}

const joinParts = (v: unknown, sep: string) =>
  typeof v === "string" ? v : (Array.isArray(v) ? v.map((p) => (typeof p === "string" ? p : str(isObject(p) ? p.value : ""))).join(sep) : "");

function resolveUrl(url: PostmanRequest["url"]): string {
  if (typeof url === "string") return fromPostman(url);
  if (!isObject(url)) return "";
  let raw = str(url.raw);
  if (!raw) {
    raw = joinParts(url.host, ".") + (url.port ? `:${url.port}` : "");
    const path = joinParts(url.path, "/").replace(/^\//, "");
    if (path) raw += `/${path}`;
    if (url.protocol && raw) raw = `${url.protocol}://${raw}`;
  }
  // The query array is authoritative; raw repeats it unencoded.
  const query = list<KV>(url.query).filter((q) => q.key && !q.disabled);
  if (Array.isArray(url.query)) {
    raw = raw.split("?")[0];
    const enc = (v: unknown) => encodeURIComponent(fromPostman(str(v))).replace(/%3C%3C(\w+)%3E%3E/g, "<<$1>>");
    if (query.length) raw += "?" + query.map((q) => `${enc(q.key)}=${enc(q.value)}`).join("&");
  }
  return fromPostman(raw);
}

function kvBody(kvs: KV[]): Record<string, string> {
  const obj: Record<string, string> = {};
  for (const p of kvs) {
    if (!p.key || p.disabled) continue;
    if (p.type === "file") {
      const src = Array.isArray(p.src) ? p.src[0] : p.src;
      obj[p.key] = src ? `@${src}` : "@filename";
    } else obj[p.key] = fromPostman(str(p.value));
  }
  return obj;
}

function buildCurl(req: PostmanRequest, inheritedAuth: Record<string, string> | null): string {
  const method = str(req.method || "GET").toUpperCase();
  const url = resolveUrl(req.url);
  const headers: Record<string, string> = {};

  // The request's own auth wins; "noauth" opts out of the inherited one.
  const own = req.auth;
  const auth = !isObject(own) || !own.type || own.type === "inherit" ? inheritedAuth : authHeaders(own);
  Object.assign(headers, auth);

  for (const h of list<KV>(req.header)) {
    if (h.key && !h.disabled) headers[h.key] = fromPostman(str(h.value));
  }

  let body = "";
  const b = isObject(req.body) ? req.body : undefined;
  if (b?.mode === "raw") body = fromPostman(str(b.raw));
  else if (b?.mode === "urlencoded" || b?.mode === "formdata") {
    const obj = kvBody(list<KV>(b.mode === "urlencoded" ? b.urlencoded : b.formdata));
    if (Object.keys(obj).length) body = JSON.stringify(obj, null, "\t");
    if (b.mode === "urlencoded") headers["Content-Type"] ??= "application/x-www-form-urlencoded";
  }
  try {
    body = JSON.stringify(JSON.parse(body), null, "\t");
  } catch {
    // not JSON: keep the raw text
  }

  return jsonToCurl({ method, url, headers: JSON.stringify(headers), body, params: "{}" });
}

function addVariables(vars: unknown, env: Record<string, string>): void {
  for (const v of list<KV>(vars)) {
    if (v.key && v.enabled !== false) env[v.key] = str(v.value);
  }
}

function convertItems(items: unknown, auth: Record<string, string> | null, env: Record<string, string>): THittableItem[] {
  const result: THittableItem[] = [];
  for (const item of list<PostmanItem>(items)) {
    if (item.disabled === true) continue;
    addVariables(item.variable, env);
    if (Array.isArray(item.item)) {
      const folderAuth = isObject(item.auth) && item.auth.type && item.auth.type !== "inherit" ? authHeaders(item.auth) : auth;
      result.push({ type: "folder", name: item.name || "Unnamed Folder", items: convertItems(item.item, folderAuth, env) });
    } else if (typeof item.request === "string") {
      result.push({ type: "route", name: item.name || "Unnamed", curl: jsonToCurl({ method: "GET", url: fromPostman(item.request), headers: "{}", body: "", params: "{}" }), response: "" });
    } else if (isObject(item.request)) {
      result.push({ type: "route", name: item.name || "Unnamed", curl: buildCurl(item.request, auth), response: "" });
    }
  }
  return result;
}

function scanItem(item: THittableItem, env: Record<string, string>): void {
  if (item.type === "route") addMissingVars(item.curl, env);
  else item.items.forEach((child) => scanItem(child, env));
}

function processCollection(col: PostmanCollection): THittableCollection {
  const env: Record<string, string> = {};
  addVariables(col.variable, env);
  const items = convertItems(col.item, authHeaders(col.auth), env);
  items.forEach((item) => scanItem(item, env));
  return { collectionName: str(col.info?.name) || "Imported Collection", items, env, secrets: {} };
}

export function parsePostmanCollection(json: unknown): THittableCollection[] {
  const cols = Array.isArray(json) ? json : [json];
  return cols.filter((c): c is PostmanCollection => isObject(c) && isObject(c.info)).map(processCollection);
}

export function isPostmanCollection(json: unknown): boolean {
  if (!isObject(json) || !isObject(json.info)) return false;
  const info = json.info;
  if (typeof info.schema === "string" && info.schema.includes("schema.getpostman.com")) return true;
  return typeof info.name === "string" && Array.isArray(json.item);
}
