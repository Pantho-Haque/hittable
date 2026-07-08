import { THittableCollection, THittableItem } from "@/types";

interface InsomniaExport {
  _type?: string;
  resources?: InsomniaResource[];
}

interface InsomniaResource {
  _type?: string;
  _id?: string;
  parentId?: string;
  name?: string;
  url?: string;
  method?: string;
  headers?: Record<string, string>;
  body?: { mimeType?: string; text?: string };
  data?: Record<string, string>;
  environment?: Record<string, string>;
  _kvPairData?: { name: string; value: string; enabled: boolean }[];
}

function resolveVariableSyntax(text: string): string {
  return text.replace(/\{\{\s*_\.(\w+)\s*\}\}/g, "<<$1>>");
}

function parseInsomniaRequest(req: InsomniaResource): { name: string; curl: string; response: string } {
  const method = req.method ?? "GET";
  const url = resolveVariableSyntax(req.url ?? "");
  const headers = req.headers ?? {};
  const body = req.body;

  let curl = `curl -X ${method} ${url}`;

  for (const [k, v] of Object.entries(headers)) {
    curl += ` -H "${k}: ${resolveVariableSyntax(v)}"`;
  }

  const noBody = ["GET", "HEAD", "DELETE"].includes(method.toUpperCase());
  if (!noBody && body?.text) {
    curl += ` -d '${resolveVariableSyntax(body.text).replace(/'/g, "'\\''")}'`;
  }

  return { name: req.name ?? "Unnamed", curl, response: "" };
}

function buildNestedItems(
  groupId: string,
  byId: Map<string, InsomniaResource>,
): THittableItem[] {
  const items: THittableItem[] = [];

  // Find child request groups (sub-folders)
  const childGroups = Array.from(byId.values()).filter(
    (r) => r._type === "request_group" && r.parentId === groupId,
  );
  for (const group of childGroups) {
    items.push({
      type: "folder",
      name: group.name ?? "Unnamed Folder",
      items: buildNestedItems(group._id ?? "", byId),
    });
  }

  // Find child requests
  const childRequests = Array.from(byId.values()).filter(
    (r) => r._type === "request" && r.parentId === groupId,
  );
  for (const req of childRequests) {
    const parsed = parseInsomniaRequest(req);
    items.push({
      type: "route",
      name: parsed.name,
      curl: parsed.curl,
      response: parsed.response,
    });
  }

  return items;
}

export function parseInsomniaExport(json: unknown): THittableCollection[] {
  const collections: THittableCollection[] = [];

  if (!json || typeof json !== "object") return collections;

  const data = json as InsomniaExport;
  const resources = data.resources ?? [];
  if (!resources.length) return collections;

  // Build parent-child map
  const byId = new Map<string, InsomniaResource>();
  for (const r of resources) {
    if (r._id) byId.set(r._id, r);
  }

  // Find workspace(s)
  const workspaces = resources.filter((r) => r._type === "workspace" || r._type === "export_type");

  // If no workspace, treat all request_groups as collections
  const topLevelGroups = resources.filter((r) => {
    if (r._type !== "request_group") return false;
    const parent = byId.get(r.parentId ?? "");
    return !parent || parent._type === "workspace" || parent._type === "export_type";
  });

  const envVars = extractEnvironmentVars(resources);

  if (topLevelGroups.length === 0) {
    // No folders — all requests go into one collection
    const requests = resources.filter((r) => r._type === "request");
    if (requests.length > 0) {
      const items = requests.map((r) => {
        const parsed = parseInsomniaRequest(r);
        return {
          type: "route" as const,
          name: parsed.name,
          curl: parsed.curl,
          response: parsed.response,
        };
      });
      for (const item of items) {
        scanItemForVariables(item, envVars);
      }
      collections.push({
        collectionName: workspaces[0]?.name ?? "Imported Collection",
        items,
        env: envVars,
        secrets: {},
      });
    }
  } else {
    for (const group of topLevelGroups) {
      const groupName = group.name ?? "Imported";
      const items = buildNestedItems(group._id ?? "", byId);
      for (const item of items) {
        scanItemForVariables(item, envVars);
      }
      collections.push({
        collectionName: groupName,
        items,
        env: envVars,
        secrets: {},
      });
    }
  }

  return collections;
}

function extractEnvironmentVars(resources: InsomniaResource[]): Record<string, string> {
  const envVars: Record<string, string> = {};
  const envs = resources.filter((r) => r._type === "environment");

  for (const env of envs) {
    // Try kvPairData first (Insomnia v5+)
    if (env._kvPairData?.length) {
      for (const pair of env._kvPairData) {
        if (pair.enabled && pair.name) {
          envVars[pair.name] = pair.value;
        }
      }
    }
    // Fallback to data object
    if (env.data) {
      for (const [k, v] of Object.entries(env.data)) {
        if (typeof v === "string" && k) {
          envVars[k] = v;
        }
      }
    }
  }

  return envVars;
}

function scanItemForVariables(item: THittableItem, env: Record<string, string>): void {
  if (item.type === "route") {
    for (const m of item.curl.matchAll(/<<(\w+)>>/g)) {
      if (!(m[1] in env)) {
        env[m[1]] = "";
      }
    }
  } else if (item.type === "folder") {
    for (const child of item.items) {
      scanItemForVariables(child, env);
    }
  }
}

export function isInsomniaExport(json: unknown): boolean {
  if (!json || typeof json !== "object") return false;
  const obj = json as Record<string, unknown>;
  if (obj._type === "export" || obj.__export_format === 4) return true;
  if (Array.isArray(obj.resources) && obj.resources.length > 0) {
    const first = obj.resources[0] as Record<string, unknown>;
    if (first?._type) return true;
  }
  return false;
}
