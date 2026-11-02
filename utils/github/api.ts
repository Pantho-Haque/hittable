// Typed client for /api/github/*. Every failure becomes a GitHubError whose
// `code` is the server's stable error code (signed_out, head_moved, ...).

export type GitHubUser = { login: string; name: string | null; avatarUrl: string };

export type RepoMeta = {
  fullName: string;
  owner: string;
  name: string;
  private: boolean;
  fork: boolean;
  archived: boolean;
  canPush: boolean;
  defaultBranch: string;
  pushedAt: string | null;
  description: string | null;
};

export type Branch = { name: string; sha: string; protected: boolean };
export type TreeEntry = { path: string; type: "blob" | "tree"; sha: string; size: number };
export type Tree = { sha: string; truncated: boolean; entries: TreeEntry[] };
export type CommitInput = {
  branch: string;
  expectedHeadOid: string;
  message: string;
  additions: { path: string; contents: string }[];
  deletions: { path: string }[];
};

export class GitHubError extends Error {
  constructor(
    readonly code: string,
    readonly status: number,
    message?: string,
    readonly retryAfter?: number,
  ) {
    super(message || code);
  }
}

export const isGitHubError = (err: unknown, code?: string): err is GitHubError =>
  err instanceof GitHubError && (code === undefined || err.code === code);

/** Sign in, coming back to `next` (default: this page). */
export function signInUrl(next = typeof window === "undefined" ? "/hittable" : window.location.pathname + window.location.search) {
  return `/api/github/login?next=${encodeURIComponent(next)}`;
}

async function request(path: string, init: RequestInit = {}): Promise<Response> {
  const method = init.method ?? "GET";
  let res: Response;
  try {
    res = await fetch(`/api/github${path}`, {
      ...init,
      // Mutating routes refuse a request without this header (CSRF guard).
      headers: { ...(method !== "GET" ? { "X-Hittable": "1", "Content-Type": "application/json" } : {}), ...init.headers },
      cache: init.cache ?? "no-store",
    });
  } catch {
    throw new GitHubError("network", 0, "Could not reach the server. Check your connection.");
  }
  if (res.ok || res.status === 304) return res;
  let body: { error?: string; message?: string; retryAfter?: number } = {};
  try {
    body = await res.json();
  } catch {
    // not JSON
  }
  const retry = body.retryAfter ?? (Number(res.headers.get("retry-after")) || undefined);
  throw new GitHubError(body.error ?? "upstream", res.status, body.message, retry);
}

const getJson = async <T>(path: string, init?: RequestInit) => (await request(path, init)).json() as Promise<T>;
const post = <T>(path: string, body: unknown) => getJson<T>(path, { method: "POST", body: JSON.stringify(body) });
const repoPath = (owner: string, repo: string) => `/repos/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}`;

let reposCache: Promise<RepoMeta[]> | null = null;

export const api = {
  async session(): Promise<GitHubUser | null> {
    try {
      return (await getJson<{ user: GitHubUser | null }>("/session")).user;
    } catch {
      return null; // not configured or unreachable reads as signed out
    }
  },
  /** Cached for the page's life; `fresh` refetches. */
  repos(fresh = false): Promise<RepoMeta[]> {
    if (fresh || !reposCache) {
      reposCache = getJson<RepoMeta[]>("/repos");
      reposCache.catch(() => (reposCache = null));
    }
    return reposCache;
  },
  branches: (o: string, r: string) => getJson<Branch[]>(`${repoPath(o, r)}/branches`),
  createBranch: (o: string, r: string, name: string, fromSha: string) =>
    post<{ name: string; sha: string }>(`${repoPath(o, r)}/branches`, { name, fromSha }),
  /** null when unchanged since `etag` (304). */
  async head(o: string, r: string, branch: string, etag?: string): Promise<{ sha: string; etag: string | null } | null> {
    const res = await request(`${repoPath(o, r)}/head?branch=${encodeURIComponent(branch)}`, {
      headers: etag ? { "If-None-Match": etag } : {},
    });
    if (res.status === 304) return null;
    return { sha: ((await res.json()) as { sha: string }).sha, etag: res.headers.get("etag") };
  },
  tree: (o: string, r: string, sha: string, path?: string) =>
    getJson<Tree>(`${repoPath(o, r)}/tree?sha=${sha}${path ? `&path=${encodeURIComponent(path)}` : ""}`),
  async blob(o: string, r: string, sha: string): Promise<Uint8Array> {
    const res = await request(`${repoPath(o, r)}/blob/${sha}`, { cache: "default" });
    return new Uint8Array(await res.arrayBuffer());
  },
  commit: (o: string, r: string, input: CommitInput) => post<{ sha: string; url: string }>(`${repoPath(o, r)}/commit`, input),
  pull: (o: string, r: string, input: { head: string; base: string; title: string; body: string }) =>
    post<{ number: number; url: string }>(`${repoPath(o, r)}/pulls`, input),
  compare: (o: string, r: string, base: string, head: string) =>
    getJson<{ files: string[]; truncated: boolean }>(
      `${repoPath(o, r)}/compare?base=${encodeURIComponent(base)}&head=${encodeURIComponent(head)}`,
    ),
  logout: () => post<{ ok: boolean }>("/logout", {}),
};

/** A message a person can act on, for any GitHubError. */
export function describeError(err: unknown): string {
  if (!(err instanceof GitHubError)) return (err as Error)?.message || "Something went wrong.";
  switch (err.code) {
    case "signed_out":
      return "Your GitHub session ended. Sign in again to continue; your changes are kept on this device.";
    case "rate_limited":
      return `GitHub's rate limit was reached. Try again in ${err.retryAfter ?? 60} seconds.`;
    case "forbidden":
      return "GitHub refused this: you may not have access. Organisations may need to approve Hittable.";
    case "not_found":
      return "Not found on GitHub, or you don't have access to it.";
    case "workflow_scope":
      return "Hittable can't change GitHub Actions workflows (.github/workflows). Edit them on GitHub.";
    case "too_large":
      return "This file is too large to load here.";
    case "network":
      return err.message;
    default:
      return err.message && err.message !== err.code ? err.message : "GitHub didn't complete the request. Please try again.";
  }
}
