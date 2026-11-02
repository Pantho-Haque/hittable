import { THittableCurlJson, THittableEnv } from "@/types";
import { resolveEnv } from "./resolveEnv";
import { buildUploadBody, bytesToBase64, formatBytes, needsUploadBody, type ReadFile } from "./uploadBody";

/** What a request with file uploads needs: where the .hit is, and how to read workspace files. */
export type SendContext = {
  hitPath?: string[];
  readFile?: ReadFile;
  /** The extension can carry a binary body (1.1.0 and later). */
  extensionBinary?: boolean;
};

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
  signal?: AbortSignal,
  bodyBase64?: string,
): Promise<{ data: unknown; status: number; ok: boolean; error?: string }> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) return reject(abortError());
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
    const onAbort = () => {
      cleanup();
      reject(abortError());
    };
    const cleanup = () => {
      clearTimeout(timer);
      window.removeEventListener("message", handler);
      signal?.removeEventListener("abort", onAbort);
    };

    signal?.addEventListener("abort", onAbort);
    window.addEventListener("message", handler);
    const payload = bodyBase64 === undefined ? { url, method, headers, body } : { url, method, headers, body: null, bodyBase64 };
    window.postMessage({ type: "HITTABLE_REQUEST", id, payload }, window.location.origin);
  });
}

function abortError() {
  return new DOMException("Request cancelled", "AbortError");
}

/** True for the error a cancelled request rejects with. */
export function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

// The server gives up after PROXY_TIMEOUT_MS (30 s by default); this is only a
// backstop for a proxy that never answers at all.
const CLIENT_TIMEOUT_MS = 65_000;

async function fetchViaProxy(
  url: string,
  method: string,
  headers: string,
  body: unknown,
  signal?: AbortSignal,
  upload?: Uint8Array<ArrayBuffer>,
) {
  // One controller for both the user's Cancel and the backstop timeout.
  // (AbortSignal.any looked neater, but WebKit's fetch ignored the composed
  // signal and ran the request to the end.)
  const controller = new AbortController();
  let timedOut = false;
  const timer = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, CLIENT_TIMEOUT_MS);
  const onAbort = () => controller.abort();
  if (signal?.aborted) controller.abort();
  signal?.addEventListener("abort", onAbort);
  const done = () => {
    clearTimeout(timer);
    signal?.removeEventListener("abort", onAbort);
  };
  let response: Response;
  try {
    response = await fetch("/api/proxy", {
      method: "POST",
      signal: controller.signal,
      credentials: "include",
      ...(upload ? uploadInit(url, method, headers, upload) : {
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url, method, headers, body }),
      }),
    });
  } catch (err) {
    done();
    if (signal?.aborted) throw abortError();
    if (timedOut) throw new Error(`No answer from the proxy within ${CLIENT_TIMEOUT_MS / 1000} s`);
    throw err;
  }

  // Vercel answers an oversized body with its own page, not the proxy's JSON.
  if (response.status === 413 && upload) {
    done();
    return {
      data: null,
      status: 413,
      statusText: "Payload Too Large",
      ok: false,
      error: `The upload is ${formatBytes(upload.byteLength)}, more than the proxy accepts (4 MB unless PROXY_MAX_REQUEST_BYTES raises it on a self-hosted server). Send larger files from the terminal app.`,
    };
  }

  let responseData;
  try {
    responseData = await response.json();
  } catch {
    done();
    if (signal?.aborted) throw abortError(); // cancelled while the body was arriving
    return {
      data: null,
      status: response.status,
      statusText: response.statusText,
      ok: response.ok,
      error: "Failed to parse proxy response",
    };
  }
  done();
  return responseData;
}

/** The proxy's upload envelope: the request as JSON, the body as a file part. */
function uploadInit(url: string, method: string, headers: string, bytes: Uint8Array<ArrayBuffer>): RequestInit {
  const form = new FormData();
  form.append("request", JSON.stringify({ url, method, headers }));
  form.append("body", new Blob([bytes]));
  return { headers: { "X-Hittable": "1" }, body: form };
}

function parseHeaders(headers: string): Record<string, string> {
  try {
    const h = JSON.parse(headers || "{}");
    return h && typeof h === "object" && !Array.isArray(h) ? h : {};
  } catch {
    return {};
  }
}

export async function hittableProxy(
  formInput: THittableCurlJson,
  env: THittableEnv | undefined,
  secrets: THittableEnv | undefined,
  extensionAvailable: boolean,
  signal?: AbortSignal,
  ctx: SendContext = {},
) {
  const mergedEnv = { ...(env ?? {}), ...(secrets ?? {}) };
  const {url, method, headers, body} = resolveEnv(formInput, Object.keys(mergedEnv).length > 0 ? mergedEnv : undefined);

  const isLocal = isLocalUrl(url);
  if (isLocal && !extensionAvailable) {
    throw new Error("Install the Hittable Extension to make localhost requests");
  }

  // Files are read only when the method sends a body.
  const sendsBody = !["GET", "HEAD"].includes(method.toUpperCase());
  const upload = sendsBody && needsUploadBody(body, parseHeaders(headers)) ? await buildUploadBody(body, parseHeaders(headers), ctx) : null;
  const text = upload?.kind === "text" ? upload.body : body;

  if (isLocal) {
    if (upload?.kind === "bytes") {
      if (!ctx.extensionBinary) throw new Error("Update the Hittable extension (1.1.0 or later) to upload files to localhost.");
      return fetchViaExtension(url, method, upload.headers, null, undefined, signal, bytesToBase64(upload.bytes));
    }
    return fetchViaExtension(url, method, parseHeaders(headers), text, undefined, signal);
  }

  if (upload?.kind === "bytes") return fetchViaProxy(url, method, JSON.stringify(upload.headers), null, signal, upload.bytes);
  return fetchViaProxy(url, method, headers, text, signal);
}
