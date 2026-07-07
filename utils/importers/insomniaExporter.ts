import { THittableCollection, THittableItem } from "@/types";

function resolveEnvSyntax(text: string): string {
  return text.replace(/<<(\w+)>>/g, "{{ _.${1} }}");
}

function generateId(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

function parseCurlString(curlStr: string): {
  method: string;
  url: string;
  headers: Record<string, string>;
  body: string;
} {
  const methodMatch = curlStr.match(/-X\s+(\w+)/);
  const method = methodMatch?.[1] ?? "GET";

  const urlMatch = curlStr.match(/(?:curl\s+(?:-X\s+\w+\s+)?)(https?:\/\/[^\s]+)/);
  const url = urlMatch?.[1] ?? "";

  const headers: Record<string, string> = {};
  const headerRegex = /-H\s+"([^"]+):([^"]+)"/g;
  let match;
  while ((match = headerRegex.exec(curlStr)) !== null) {
    headers[match[1].trim()] = match[2].trim();
  }

  const bodyMatch = curlStr.match(/-d\s+'([\s\S]*?)'/);
  const body = bodyMatch?.[1] ?? "";

  return { method, url, headers, body };
}

function itemsToInsomniaResources(
  items: THittableItem[],
  parentId: string,
  resources: Record<string, unknown>[],
): void {
  for (const item of items) {
    if (item.type === "folder") {
      const groupId = generateId("fld");
      resources.push({
        _type: "request_group",
        _id: groupId,
        parentId,
        name: item.name,
      });
      itemsToInsomniaResources(item.items, groupId, resources);
    } else {
      const parsed = parseCurlString(item.curl);
      const requestId = generateId("req");

      const insomniaHeaders: Record<string, string> = {};
      for (const [k, v] of Object.entries(parsed.headers)) {
        insomniaHeaders[resolveEnvSyntax(k)] = resolveEnvSyntax(v);
      }

      resources.push({
        _type: "request",
        _id: requestId,
        parentId,
        name: item.name,
        method: parsed.method,
        url: resolveEnvSyntax(parsed.url),
        headers: insomniaHeaders,
        body: {
          mimeType: "application/json",
          text: parsed.body ? resolveEnvSyntax(parsed.body) : "",
        },
      });
    }
  }
}

export function exportToInsomniaCollection(
  collection: THittableCollection,
): string {
  const workspaceId = generateId("wrk");
  const envId = generateId("env");
  const rootGroupId = generateId("fld");

  const resources: Record<string, unknown>[] = [];

  // Workspace
  resources.push({
    _type: "workspace",
    _id: workspaceId,
    parentId: "__PROJECT_ID__",
    name: collection.collectionName,
    description: "",
    scope: "collection",
  });

  // Base environment
  resources.push({
    _type: "environment",
    _id: envId,
    parentId: workspaceId,
    name: "Base Environment",
    data: collection.env,
  });

  // Root request group
  resources.push({
    _type: "request_group",
    _id: rootGroupId,
    parentId: workspaceId,
    name: collection.collectionName,
  });

  // Items (folders and requests)
  itemsToInsomniaResources(collection.items, rootGroupId, resources);

  return JSON.stringify({ resources }, null, 2);
}
