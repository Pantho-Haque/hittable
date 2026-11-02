import { curlConverter } from "@/utils/curlConverter";

// FR-G10: the removed "Local" mode kept collections in localStorage.hittable.
// Nothing reads them any more except this one-time download as Postman v2.1.

export const LEGACY_KEY = "hittable";
export const LEGACY_DISMISSED_KEY = "hittable_legacy_dismissed";

type LegacyRoute = { type: "route"; name: string; curl: string; response?: unknown };
type LegacyFolder = { type: "folder"; name: string; items: LegacyItem[] };
type LegacyItem = LegacyRoute | LegacyFolder;
export type LegacyCollection = {
  collectionName: string;
  items: LegacyItem[];
  env?: Record<string, string>;
  secrets?: Record<string, string>;
};

/** Old collections, newest format and the older `curls` one alike; null when there are none or the banner was dismissed. */
export function readLegacyCollections(): LegacyCollection[] | null {
  try {
    if (localStorage.getItem(LEGACY_DISMISSED_KEY)) return null;
    const parsed = JSON.parse(localStorage.getItem(LEGACY_KEY) ?? "null");
    if (!Array.isArray(parsed)) return null;
    const collections = parsed
      .filter((c) => c && typeof c === "object" && typeof c.collectionName === "string")
      .map((c): LegacyCollection => ({
        collectionName: c.collectionName,
        items: Array.isArray(c.items)
          ? c.items
          : Array.isArray(c.curls)
            ? c.curls.map((r: { name: string; curl: string }) => ({ type: "route", name: r.name, curl: r.curl }))
            : [],
        env: c.env,
        secrets: c.secrets,
      }));
    return collections.length ? collections : null;
  } catch {
    return null; // storage blocked or corrupt
  }
}

export function dismissLegacy() {
  try {
    localStorage.setItem(LEGACY_DISMISSED_KEY, "1");
  } catch {
    // storage blocked: the banner comes back next time
  }
}

const toPostman = (s: string) => s.replace(/<<(\w+)>>/g, "{{$1}}");

function headers(json: string): { key: string; value: string }[] {
  try {
    const parsed = JSON.parse(json);
    if (!parsed || typeof parsed !== "object") return [];
    return Object.entries(parsed).map(([key, value]) => ({ key, value: toPostman(String(value)) }));
  } catch {
    return [];
  }
}

function items(list: LegacyItem[]): Record<string, unknown>[] {
  return list.map((item) => {
    if (item.type === "folder") return { name: item.name, item: items(item.items ?? []) };
    const parsed = curlConverter(item.curl ?? "");
    const request: Record<string, unknown> = { method: parsed.method, header: headers(parsed.headers), url: toPostman(parsed.url) };
    const noBody = ["GET", "HEAD", "DELETE"].includes(parsed.method);
    if (!noBody && parsed.body && parsed.body !== "{}") request.body = { mode: "raw", raw: toPostman(parsed.body) };
    return { name: item.name, request };
  });
}

/**
 * A Postman v2.1 collection: <<KEY>> becomes {{KEY}} and env values travel as
 * collection variables. Secret values are never exported: their keys appear
 * with an empty value.
 */
export function toPostmanCollection(collection: LegacyCollection): string {
  const env = collection.env ?? {};
  const variable = Object.entries(env).map(([key, value]) => ({ key, value: String(value), type: "string" }));
  for (const key of Object.keys(collection.secrets ?? {})) {
    if (!(key in env)) variable.push({ key, value: "", type: "string" });
  }
  return JSON.stringify(
    {
      info: { name: collection.collectionName, schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" },
      item: items(collection.items),
      variable,
    },
    null,
    2,
  );
}

/** Saves one .postman_collection.json per collection. */
export function downloadLegacyCollections(collections: LegacyCollection[]) {
  for (const c of collections) {
    const url = URL.createObjectURL(new Blob([toPostmanCollection(c)], { type: "application/json" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = `${c.collectionName.replace(/[^\w.-]+/g, "-") || "collection"}.postman_collection.json`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000); // after the download has started
  }
}
