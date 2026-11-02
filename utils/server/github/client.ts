import type { GithubConfig } from "./config";

// Every failure a /api/github/* route returns is one of these: the HTTP status
// plus a stable `error` code the UI switches on. `message` is GitHub's own text
// (never contains a token) or ours.
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message?: string,
    readonly extra: Record<string, unknown> = {},
  ) {
    super(message ?? code);
  }
}

export function json(body: unknown, status = 200, headers: Record<string, string> = {}): Response {
  return Response.json(body, { status, headers: { "Cache-Control": "no-store", ...headers } });
}

export function errorResponse(err: ApiError): Response {
  const body: Record<string, unknown> = { error: err.code, ...err.extra };
  if (err.message !== err.code) body.message = err.message;
  const headers: Record<string, string> = {};
  if (typeof err.extra.retryAfter === "number") headers["Retry-After"] = String(err.extra.retryAfter);
  return json(body, err.status, headers);
}

export type Gh = { cfg: GithubConfig; token: string };

const BASE_HEADERS = {
  "X-GitHub-Api-Version": "2022-11-28",
  "User-Agent": "hittable",
};

// Maps a non-2xx GitHub reply to an ApiError.
export async function toApiError(res: Response): Promise<ApiError> {
  let message = "";
  try {
    message = String(((await res.json()) as { message?: unknown }).message ?? "");
  } catch {
    // not JSON
  }
  const retryHeader = Number(res.headers.get("retry-after"));
  const remaining = res.headers.get("x-ratelimit-remaining");
  const isRateLimit =
    res.status === 429 ||
    (res.status === 403 && (res.headers.has("retry-after") || remaining === "0" || /rate limit/i.test(message)));
  if (res.status === 401) return new ApiError(401, "signed_out");
  if (isRateLimit) {
    const reset = Number(res.headers.get("x-ratelimit-reset"));
    const retryAfter =
      Number.isFinite(retryHeader) && retryHeader > 0
        ? Math.ceil(retryHeader)
        : remaining === "0" && reset > 0
          ? Math.max(1, reset - Math.floor(Date.now() / 1000))
          : 60;
    return new ApiError(429, "rate_limited", message || undefined, { retryAfter });
  }
  if (res.status === 403) return new ApiError(403, "forbidden", message || undefined);
  if (res.status === 404) return new ApiError(404, "not_found", message || undefined);
  if (res.status === 409) return new ApiError(409, /empty/i.test(message) ? "empty_repo" : "conflict", message || undefined);
  if (res.status === 422) return new ApiError(422, "invalid", message || undefined);
  return new ApiError(502, "upstream", message || `GitHub answered ${res.status}`);
}

type GhInit = { method?: string; body?: unknown; accept?: string; headers?: Record<string, string> };

// Calls the GitHub API. Returns the Response when it is 2xx or 304, throws ApiError otherwise.
// Only the configured API/GraphQL hosts are reachable: a Link header pointing
// elsewhere would otherwise carry the token off-site.
export async function ghFetch(gh: Gh, pathOrUrl: string, init: GhInit = {}): Promise<Response> {
  const url = /^https?:/.test(pathOrUrl) ? pathOrUrl : gh.cfg.apiUrl + pathOrUrl;
  if (!url.startsWith(gh.cfg.apiUrl + "/") && url !== gh.cfg.graphqlUrl) {
    throw new ApiError(502, "upstream", "Refused to follow a GitHub URL outside the API");
  }
  let res: Response;
  try {
    res = await fetch(url, {
      method: init.method ?? "GET",
      headers: {
        ...BASE_HEADERS,
        Accept: init.accept ?? "application/vnd.github+json",
        Authorization: `Bearer ${gh.token}`,
        ...(init.body !== undefined ? { "Content-Type": "application/json" } : {}),
        ...init.headers,
      },
      body: init.body !== undefined ? JSON.stringify(init.body) : undefined,
      cache: "no-store",
    });
  } catch {
    throw new ApiError(502, "upstream", "Could not reach GitHub");
  }
  if (res.ok || res.status === 304) return res;
  throw await toApiError(res);
}

export async function ghJson<T>(gh: Gh, path: string, init?: GhInit): Promise<T> {
  return (await ghFetch(gh, path, init)).json() as Promise<T>;
}

export function nextLink(link: string | null): string | null {
  return link?.match(/<([^>]+)>\s*;\s*rel="next"/)?.[1] ?? null;
}

// Follows rel="next" until done or `cap` items.
export async function ghPages<T>(gh: Gh, path: string, cap = 1000): Promise<T[]> {
  const out: T[] = [];
  let url: string | null = path;
  while (url && out.length < cap) {
    const res = await ghFetch(gh, url);
    out.push(...((await res.json()) as T[]));
    url = nextLink(res.headers.get("link"));
  }
  return out.slice(0, cap);
}

export type GraphqlError = { type?: string; message: string };

export async function graphql<T>(gh: Gh, query: string, variables: unknown): Promise<{ data?: T; errors?: GraphqlError[] }> {
  return ghJson(gh, gh.cfg.graphqlUrl, { method: "POST", body: { query, variables }, accept: "application/json" });
}

export type TokenSet = { t: string; rt?: string; exp?: number };

// POST /login/oauth/access_token, for both the code exchange and a refresh.
// GitHub answers 200 with {error} on failure; null then.
export async function tokenRequest(cfg: GithubConfig, params: Record<string, string>): Promise<TokenSet | null> {
  try {
    const res = await fetch(`${cfg.oauthUrl}/login/oauth/access_token`, {
      method: "POST",
      headers: { Accept: "application/json", "Content-Type": "application/x-www-form-urlencoded", "User-Agent": "hittable" },
      body: new URLSearchParams({ client_id: cfg.clientId, client_secret: cfg.clientSecret, ...params }),
      cache: "no-store",
    });
    if (!res.ok) return null;
    const body = (await res.json()) as { access_token?: string; refresh_token?: string; expires_in?: number };
    if (!body.access_token) return null;
    return {
      t: body.access_token,
      rt: body.refresh_token || undefined,
      exp: body.expires_in ? Math.floor(Date.now() / 1000) + Number(body.expires_in) : undefined,
    };
  } catch {
    return null;
  }
}
