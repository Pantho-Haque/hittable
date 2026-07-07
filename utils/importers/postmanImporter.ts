import { THittableCollection, THittableItem } from "@/types";

interface PostmanCollection {
  info?: { name?: string; schema?: string };
  item?: (PostmanItem | PostmanFolder)[];
  variable?: PostmanVariable[];
  auth?: PostmanAuth;
}

interface PostmanItem {
  name?: string;
  request?: PostmanRequest | string;
  response?: unknown[];
}

interface PostmanFolder {
  name?: string;
  item?: (PostmanItem | PostmanFolder)[];
  auth?: PostmanAuth;
}

interface PostmanRequest {
  method?: string;
  url?: string | { raw?: string; host?: string | string[]; path?: (string | { value: string })[]; query?: { key?: string; value?: string; disabled?: boolean }[] };
  header?: { key?: string; value?: string; disabled?: boolean }[];
  body?: {
    mode?: "raw" | "urlencoded" | "formdata" | "file" | "graphql";
    raw?: string;
    urlencoded?: { key?: string; value?: string; disabled?: boolean }[];
    formdata?: { key?: string; value?: string; type?: "text" | "file"; src?: string | string[] | null; disabled?: boolean }[];
  };
  auth?: PostmanAuth;
}

interface PostmanVariable {
  key?: string;
  value?: string;
  enabled?: boolean;
}

type PostmanAuth = {
  type?: string;
  basic?: { key: string; value: string }[];
  bearer?: { key: string; value: string }[];
  apikey?: { key: string; value: string }[];
} | null;

function resolveUrl(url: PostmanRequest["url"]): string {
  if (!url) return "";
  if (typeof url === "string") return url;
  if (url.raw) return url.raw;
  const host = Array.isArray(url.host) ? url.host.join(".") : (url.host ?? "");
  const path = (url.path ?? []).map((p) => (typeof p === "string" ? p : p.value)).join("/");
  return `${host}/${path}`;
}

function resolveVariableSyntax(text: string): string {
  return text.replace(/\{\{(\w+)\}\}/g, "<<$1>>");
}

function headersToJSON(headers: PostmanRequest["header"]): string {
  if (!headers?.length) return "{}";
  const obj: Record<string, string> = {};
  for (const h of headers) {
    if (h.key && !h.disabled) {
      obj[h.key] = resolveVariableSyntax(h.value ?? "");
    }
  }
  return JSON.stringify(obj, null, "\t");
}

function bodyToJSON(body: PostmanRequest["body"]): string {
  if (!body) return "{}";
  switch (body.mode) {
    case "raw": {
      if (!body.raw) return "{}";
      try {
        const parsed = JSON.parse(body.raw);
        return JSON.stringify(parsed, null, "\t");
      } catch {
        return body.raw;
      }
    }
    case "urlencoded": {
      if (!body.urlencoded?.length) return "{}";
      const obj: Record<string, string> = {};
      for (const p of body.urlencoded) {
        if (p.key && !p.disabled) obj[p.key] = resolveVariableSyntax(p.value ?? "");
      }
      return JSON.stringify(obj, null, "\t");
    }
    case "formdata": {
      if (!body.formdata?.length) return "{}";
      const obj: Record<string, string> = {};
      for (const p of body.formdata) {
        if (p.key && !p.disabled) {
          if (p.type === "file") {
            const src = Array.isArray(p.src) ? p.src[0] : p.src;
            obj[p.key] = src ? `@${src}` : "@filename";
          } else {
            obj[p.key] = resolveVariableSyntax(p.value ?? "");
          }
        }
      }
      return JSON.stringify(obj, null, "\t");
    }
    default:
      return body.raw ? body.raw : "{}";
  }
}

function buildCurlFromRequest(req: PostmanRequest): string {
  const method = req.method ?? "GET";
  const url = resolveUrl(req.url);
  const headers = headersToJSON(req.header);
  const body = bodyToJSON(req.body);

  let curl = `curl -X ${method} ${url}`;
  try {
    const parsed = JSON.parse(headers);
    for (const [k, v] of Object.entries(parsed)) {
      curl += ` -H "${k}: ${v}"`;
    }
  } catch { /* skip */ }
  const noBody = ["GET", "HEAD", "DELETE"].includes(method.toUpperCase());
  if (body && body !== "{}" && !noBody) {
    curl += ` -d '${body.replace(/'/g, "'\\''")}'`;
  }
  return curl;
}

function convertPostmanItems(
  items: (PostmanItem | PostmanFolder)[],
): THittableItem[] {
  const result: THittableItem[] = [];
  for (const item of items) {
    if ("item" in item && Array.isArray(item.item)) {
      // This is a folder — create real nested folder
      result.push({
        type: "folder",
        name: item.name ?? "Unnamed Folder",
        items: convertPostmanItems(item.item),
      });
    } else if ("request" in item) {
      const req = item.request;
      if (typeof req === "string") {
        result.push({
          type: "route",
          name: item.name ?? "Unnamed",
          curl: `curl ${req}`,
          response: "",
        });
      } else if (req) {
        result.push({
          type: "route",
          name: item.name ?? "Unnamed",
          curl: buildCurlFromRequest(req),
          response: "",
        });
      }
    }
  }
  return result;
}

function collectVariables(
  items: (PostmanItem | PostmanFolder)[],
): Record<string, string> {
  const vars: Record<string, string> = {};
  for (const item of items) {
    if ("variable" in item && Array.isArray(item.variable)) {
      for (const v of item.variable) {
        if (v.key && v.value && v.enabled !== false) {
          vars[v.key] = v.value;
        }
      }
    }
    if ("item" in item && Array.isArray(item.item)) {
      Object.assign(vars, collectVariables(item.item));
    }
  }
  return vars;
}

export function parsePostmanCollection(json: unknown): THittableCollection[] {
  const collections: THittableCollection[] = [];

  function processCollection(col: PostmanCollection): THittableCollection {
    const collectionName = col.info?.name ?? "Imported Collection";
    const items = col.item ?? [];
    const envVars: Record<string, string> = {};

    // Collection-level variables
    if (col.variable?.length) {
      for (const v of col.variable) {
        if (v.key && v.value && v.enabled !== false) {
          envVars[v.key] = v.value;
        }
      }
    }

    // Folder-level variables
    const itemVars = collectVariables(items);
    Object.assign(envVars, itemVars);

    return {
      collectionName,
      items: convertPostmanItems(items),
      env: envVars,
    };
  }

  function processTopLevel(data: PostmanCollection): void {
    const item = data.item ?? [];
    // Check if top level has actual items (requests) or just folders
    const hasTopLevelRequests = item.some((i) => "request" in i);
    const hasFolders = item.some((i) => "item" in i);

    if (hasTopLevelRequests || !hasFolders) {
      collections.push(processCollection(data));
    } else if (hasFolders) {
      // Each top-level folder becomes a collection
      for (const folder of item) {
        if ("item" in folder && Array.isArray(folder.item)) {
          const folderItems = folder.item;
          const subCollection: PostmanCollection = {
            info: { name: folder.name ?? "Imported", schema: data.info?.schema },
            item: folderItems,
            variable: ("variable" in folder && Array.isArray(folder.variable)) ? folder.variable : undefined,
            auth: folder.auth ?? data.auth ?? undefined,
          };
          collections.push(processCollection(subCollection));
        }
      }
    }
  }

  if (Array.isArray(json)) {
    for (const item of json) {
      if (item && typeof item === "object" && "info" in item) {
        processCollection(item as PostmanCollection);
      }
    }
  } else if (json && typeof json === "object" && "info" in json) {
    processTopLevel(json as PostmanCollection);
  }

  return collections;
}

export function isPostmanCollection(json: unknown): boolean {
  if (!json || typeof json !== "object") return false;
  const obj = json as Record<string, unknown>;
  if (obj.info && typeof obj.info === "object") {
    const info = obj.info as Record<string, unknown>;
    if (typeof info.schema === "string" && info.schema.includes("schema.getpostman.com")) return true;
    if (typeof info.name === "string" && "item" in obj && Array.isArray(obj.item)) return true;
  }
  return false;
}
