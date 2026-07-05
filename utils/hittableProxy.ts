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


export function fetchViaExtension(
  url: string,
  method: string,
  headers: Record<string, string>,
  body: string | null,
): Promise<{ data: unknown; status: number; ok: boolean; error?: string }> {
  return new Promise((resolve, reject) => {
    window.postMessage({ type: "HITTABLE_REQUEST", payload: { url, method, headers, body } }, "*");

    const handler = (event: MessageEvent) => {
      if (event.data?.type !== "HITTABLE_RESPONSE") return;
      window.removeEventListener("message", handler);
      if (event.data.success) resolve(event.data);
      else reject(new Error(event.data.error));
    };

    window.addEventListener("message", handler);

    // Timeout after 10s
    setTimeout(() => {
      window.removeEventListener("message", handler);
      reject(new Error("Extension request timed out"));
    }, 10000);
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
  extensionAvailable: boolean,
) {
  const {url, method, headers, body} = resolveEnv(formInput, env);

  let parsedHeaders: Record<string, string>;
  try {
    parsedHeaders = JSON.parse(headers || "{}");
  } catch {
    parsedHeaders = {};
  }

  const isLocal = isLocalUrl(url);
  if (isLocal && extensionAvailable) {
    return fetchViaExtension(url, method, parsedHeaders, body);
  }

  if (isLocal && !extensionAvailable) {
    throw new Error("Install the Hittable Extension to make localhost requests");
  }

  return fetchViaProxy(url, method, headers, body);
}
