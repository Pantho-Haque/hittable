import { THittableCollection, THittableItem } from "@/types";
import { curlConverter } from "@/utils/curlConverter";
import { toInsomnia } from "./variableSyntax";

// An Insomnia v4 export, shaped like the terminal app's: one workspace,
// folders as request groups, <<KEY>> as {{ _.KEY }}, variables in the base
// environment. Secret values are never exported, only their names.

function generateId(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

function isJson(text: string): boolean {
  try {
    JSON.parse(text);
    return true;
  } catch {
    return false;
  }
}

function itemsToInsomniaResources(items: THittableItem[], parentId: string, resources: Record<string, unknown>[]): void {
  for (const item of items) {
    if (item.type === "folder") {
      const groupId = generateId("fld");
      resources.push({ _type: "request_group", _id: groupId, parentId, name: item.name });
      itemsToInsomniaResources(item.items, groupId, resources);
      continue;
    }
    const parsed = curlConverter(item.curl);
    let headers: Record<string, unknown> = {};
    try {
      headers = JSON.parse(parsed.headers) ?? {};
    } catch {
      // unparseable headers export as none
    }
    const contentType = Object.entries(headers).find(([k]) => k.toLowerCase() === "content-type")?.[1];
    const noBody = ["GET", "HEAD", "DELETE"].includes(parsed.method);
    const hasBody = !noBody && parsed.body && parsed.body !== "{}";
    resources.push({
      _type: "request",
      _id: generateId("req"),
      parentId,
      name: item.name,
      method: parsed.method,
      url: toInsomnia(parsed.url),
      headers: Object.entries(headers).map(([name, value]) => ({ name, value: toInsomnia(String(value)) })),
      // Insomnia needs a mime type to show and send the body; without a
      // Content-Type header it is guessed, and comes back as that header.
      body: hasBody ? { mimeType: String(contentType ?? (isJson(parsed.body) ? "application/json" : "text/plain")), text: toInsomnia(parsed.body) } : {},
    });
  }
}

export function exportToInsomniaCollection(collection: THittableCollection): string {
  const workspaceId = generateId("wrk");
  const data: Record<string, string> = { ...(collection.env ?? {}) };
  for (const key of Object.keys(collection.secrets ?? {})) {
    if (!(key in data)) data[key] = "";
  }
  const resources: Record<string, unknown>[] = [
    { _type: "workspace", _id: workspaceId, parentId: null, name: collection.collectionName, scope: "collection" },
    { _type: "environment", _id: generateId("env"), parentId: workspaceId, name: "Base Environment", data },
  ];
  itemsToInsomniaResources(collection.items ?? [], workspaceId, resources);
  return JSON.stringify({ _type: "export", __export_format: 4, __export_source: "hittable", resources }, null, 2);
}
