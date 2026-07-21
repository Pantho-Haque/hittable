import { THitFileContent } from "@/types";

const HIT_FILE_KEYS = ["method", "url", "headers", "params", "body", "response"] as const;

export function parseHitFile(content: string): THitFileContent {
  try {
    const parsed = JSON.parse(content);
    return {
      method: parsed.method ?? "GET",
      url: parsed.url ?? "",
      headers: parsed.headers ?? {},
      params: parsed.params ?? {},
      body: parsed.body ?? "",
      response: parsed.response ?? null,
    };
  } catch {
    return {
      method: "GET",
      url: "",
      headers: {},
      params: {},
      body: "",
      response: null,
    };
  }
}

export function serializeHitFile(content: THitFileContent): string {
  const ordered: Record<string, unknown> = {};
  for (const key of HIT_FILE_KEYS) {
    ordered[key] = content[key];
  }
  return JSON.stringify(ordered, null, 2) + "\n";
}
