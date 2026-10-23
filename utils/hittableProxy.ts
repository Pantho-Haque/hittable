import { THittableCurlJson, THittableEnv } from "@/types";
import { resolveEnv } from "./hittableCollectionModifier";

const LOCAL_HOSTNAMES = ["localhost", "127.0.0.1", "0.0.0.0"];

function isLocalUrl(url: string): boolean {
  try {
    const { hostname } = new URL(url);
    return LOCAL_HOSTNAMES.some((h) => hostname === h);
  } catch {
    return false;
  }
}

let extensionRequestSeq = 0;

export function fetchViaExtension(
  url: string,
  method: string,
  headers: Record<string, string>,
  body: string | null,
  timeoutMs = 30_000,
): Promise<{ data: unknown; status: number; ok: boolean; error?: string }> {
  return new Promise((resolve, reject) => {
    // The id pairs each reply with its request, so two requests in flight
    // cannot swap responses. Extensions older than 1.0.1 do not echo it.
    const id = `${Date.now()}-${++extensionRequestSeq}`;

    const handler = (event: MessageEvent) => {
      if (event.source !== window || event.data?.type !== "HITTABLE_RESPONSE") return;
      if (event.data.id !== undefined && event.data.id !== id) return;
      cleanup();
      if (event.data.success) resolve(event.data);
      else reject(new Error(event.data.error || "Extension request failed"));
    };
    const timer = setTimeout(() => {
      cleanup();
      reject(new Error("Extension request timed out"));
    }, timeoutMs);
    const cleanup = () => {
      clearTimeout(timer);
      window.removeEventListener("message", handler);
    };

    window.addEventListener("message", handler);
    window.postMessage({ type: "HITTABLE_REQUEST", id, payload: { url, method, headers, body } }, window.location.origin);
  });
}

async function fetchViaProxy(
  url: string,
  method: string,
  headers: string,
  body: unknown,
) {
  const response = await fetch("/api/proxy", {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ url, method, headers, body }),
  });

  let responseData;
  try {
    responseData = await response.json();
  } catch {
    return {
      data: null,
      status: response.status,
      statusText: response.statusText,
      ok: response.ok,
      error: "Failed to parse proxy response",
    };
  }
  return responseData;
}

export async function hittableProxy(
  formInput: THittableCurlJson,
  env: THittableEnv | undefined,
  secrets: THittableEnv | undefined,
  extensionAvailable: boolean,
) {
  const mergedEnv = { ...(env ?? {}), ...(secrets ?? {}) };
  const {url, method, headers, body} = resolveEnv(formInput, Object.keys(mergedEnv).length > 0 ? mergedEnv : undefined);

  const isLocal = isLocalUrl(url);
  if (isLocal && extensionAvailable) {
    let parsedHeaders: Record<string, string>;
    try {
      parsedHeaders = JSON.parse(headers || "{}");
    } catch {
      parsedHeaders = {};
    }
    return fetchViaExtension(url, method, parsedHeaders, body);
  }

  if (isLocal && !extensionAvailable) {
    throw new Error("Install the Hittable Extension to make localhost requests");
  }

  return fetchViaProxy(url, method, headers, body);
}
