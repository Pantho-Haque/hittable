import { TEnvFile } from "@/types";

export function parseEnvFile(content: string): TEnvFile {
  try {
    const parsed = JSON.parse(content);
    if (typeof parsed === "object" && parsed !== null && !Array.isArray(parsed)) {
      return parsed;
    }
    return {};
  } catch {
    return {};
  }
}

export function serializeEnvFile(content: TEnvFile): string {
  return JSON.stringify(content, null, 2) + "\n";
}
