import { THittableCollection, THittableCurlJson } from "@/types";

function resolveEnvSyntax(text: string): string {
  return text.replace(/<<(\w+)>>/g, "{{$1}}");
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

function hittableToPostmanItem(
  curl: { name: string; curlJson: THittableCurlJson },
): Record<string, unknown> {
  const { curlJson, name } = curl;
  const url = resolveEnvSyntax(curlJson.url);
  const headers = headersToPostmanArray(curlJson.headers);
  const body = jsonToPostmanBody(curlJson.body);

  const item: Record<string, unknown> = {
    name,
    request: {
      method: curlJson.method,
      header: headers.map((h) => ({
        key: h.key,
        value: resolveEnvSyntax(h.value),
      })),
      url,
    },
  };

  if (body) {
    (item.request as Record<string, unknown>).body = body;
  }

  return item;
}

export function exportToPostmanCollection(
  collection: THittableCollection,
): string {
  const postmanCollection: Record<string, unknown> = {
    info: {
      name: collection.collectionName,
      schema: "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
    },
    item: collection.curls.map((curl) => {
      let parsed: THittableCurlJson;
      try {
        // Try to parse the curl string using a basic approach
        parsed = parseCurlString(curl.curl);
      } catch {
        parsed = {
          method: "GET",
          url: "",
          headers: "{}",
          body: "{}",
          params: "{}",
        };
      }
      return hittableToPostmanItem({ name: curl.name, curlJson: parsed });
    }),
    variable: Object.entries(collection.env).map(([key, value]) => ({
      key,
      value,
      type: "string",
    })),
  };

  return JSON.stringify(postmanCollection, null, 2);
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
