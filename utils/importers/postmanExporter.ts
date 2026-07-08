import { THittableCollection, THittableItem, THittableCurlJson, THittableEnv } from "@/types";

function resolveEnvValues(text: string, env: THittableEnv): string {
  return text.replace(/<<(\w+)>>/g, (_, key: string) => {
    return key in env ? env[key] : `<<${key}>>`;
  });
}

function resolveSecretSyntax(text: string): string {
  return text.replace(/<<(\w+)>>/g, "{{$1}}");
}

function resolveAllSyntax(text: string, env: THittableEnv): string {
  return text.replace(/<<(\w+)>>/g, (_, key: string) => {
    if (key in env) return `{{${key}}}`;
    return `<<${key}>>`;
  });
}

function jsonToPostmanBody(body: string): { mode: "raw"; raw: string } | undefined {
  if (!body || body === "{}") return undefined;
  return { mode: "raw", raw: body };
}

function headersToPostmanArray(headers: string): { key: string; value: string }[] {
  try {
    const parsed = JSON.parse(headers);
    if (typeof parsed !== "object" || parsed === null) return [];
    return Object.entries(parsed).map(([key, value]) => ({
      key,
      value: String(value),
    }));
  } catch {
    return [];
  }
}

function parseCurlString(curlStr: string): THittableCurlJson {
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
  const body = bodyMatch?.[1] ?? "{}";

  return {
    method,
    url,
    headers: JSON.stringify(headers, null, "\t"),
    body,
    params: "{}",
  };
}

function itemsToPostmanItems(items: THittableItem[], env: THittableEnv): Record<string, unknown>[] {
  const result: Record<string, unknown>[] = [];
  for (const item of items) {
    if (item.type === "folder") {
      result.push({
        name: item.name,
        item: itemsToPostmanItems(item.items, env),
      });
    } else {
      let parsed: THittableCurlJson;
      try {
        parsed = parseCurlString(item.curl);
      } catch {
        parsed = { method: "GET", url: "", headers: "{}", body: "{}", params: "{}" };
      }
      const url = resolveEnvValues(parsed.url, env);
      const headers = headersToPostmanArray(parsed.headers);
      const body = jsonToPostmanBody(resolveEnvValues(parsed.body, env));

      const postmanItem: Record<string, unknown> = {
        name: item.name,
        request: {
          method: parsed.method,
          header: headers.map((h) => ({
            key: h.key,
            value: resolveEnvValues(h.value, env),
          })),
          url,
        },
      };

      if (body) {
        (postmanItem.request as Record<string, unknown>).body = body;
      }

      result.push(postmanItem);
    }
  }
  return result;
}

export function exportToPostmanCollection(
  collection: THittableCollection,
): string {
  const postmanCollection: Record<string, unknown> = {
    info: {
      name: collection.collectionName,
      schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
    },
    item: itemsToPostmanItems(collection.items, collection.env),
    variable: Object.entries(collection.env).map(([key, value]) => ({
      key,
      value,
      type: "string",
    })),
  };

  // Secrets are exported as Postman variable references (not resolved)
  const secretVars = Object.entries(collection.secrets).map(([key, value]) => ({
    key,
    value,
    type: "string",
  }));
  if (secretVars.length > 0) {
    (postmanCollection.variable as Array<unknown>).push(...secretVars);
  }

  return JSON.stringify(postmanCollection, null, 2);
}
