import { THittableCollection, THittableItem } from "@/types";
import { curlConverter } from "@/utils/curlConverter";
import { toPostman } from "./variableSyntax";

// <<KEY>> becomes {{KEY}} everywhere and values travel as collection
// variables, so an export imports back unchanged. Secret values are never
// exported: secrets appear as variables with an empty value.

function headersToPostman(headers: string): { key: string; value: string }[] {
  try {
    const parsed = JSON.parse(headers);
    if (!parsed || typeof parsed !== "object") return [];
    return Object.entries(parsed).map(([key, value]) => ({ key, value: toPostman(String(value)) }));
  } catch {
    return [];
  }
}

function itemsToPostmanItems(items: THittableItem[]): Record<string, unknown>[] {
  return items.map((item) => {
    if (item.type === "folder") return { name: item.name, item: itemsToPostmanItems(item.items) };
    const parsed = curlConverter(item.curl);
    const request: Record<string, unknown> = {
      method: parsed.method,
      header: headersToPostman(parsed.headers),
      url: toPostman(parsed.url),
    };
    const noBody = ["GET", "HEAD", "DELETE"].includes(parsed.method);
    if (!noBody && parsed.body && parsed.body !== "{}") request.body = { mode: "raw", raw: toPostman(parsed.body) };
    return { name: item.name, request };
  });
}

export function exportToPostmanCollection(collection: THittableCollection): string {
  const env = collection.env ?? {};
  const variable = Object.entries(env).map(([key, value]) => ({ key, value, type: "string" }));
  for (const key of Object.keys(collection.secrets ?? {})) {
    if (!(key in env)) variable.push({ key, value: "", type: "string" });
  }
  return JSON.stringify(
    {
      info: {
        name: collection.collectionName,
        schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
      },
      item: itemsToPostmanItems(collection.items ?? []),
      variable,
    },
    null,
    2,
  );
}
