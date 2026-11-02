import { githubConfig, type GithubConfig } from "./config";
import { ApiError, errorResponse, type Gh } from "./client";
import { assertSameOrigin } from "./guard";
import { clearSessionCookie, loadSession, type Session } from "./session";

export type RouteCtx<P> = { req: Request; cfg: GithubConfig; gh: Gh; session: Session; params: P };

// Wraps a /api/github/* handler: CSRF guard (mutating), config, session (with
// refresh), and ApiError → JSON. A 401 from GitHub clears the session cookie.
export function githubRoute<P = Record<string, string>>(
  fn: (ctx: RouteCtx<P>) => Promise<Response>,
  opts: { mutating?: boolean } = {},
) {
  return async (req: Request, context?: { params: Promise<P> }): Promise<Response> => {
    let setCookie: string | undefined;
    let res: Response;
    try {
      if (opts.mutating) assertSameOrigin(req);
      const cfg = githubConfig();
      const loaded = await loadSession(req, cfg);
      setCookie = loaded.setCookie;
      if (!loaded.session) throw new ApiError(401, "signed_out");
      const session = loaded.session;
      res = await fn({ req, cfg, session, gh: { cfg, token: session.t }, params: (await context?.params) as P });
    } catch (err) {
      if (!(err instanceof ApiError)) {
        console.error("github api: unexpected failure", err instanceof Error ? err.message : "unknown");
        err = new ApiError(500, "internal", "Unexpected server error");
      }
      const e = err as ApiError;
      res = errorResponse(e);
      if (e.code === "signed_out" && req.headers.get("cookie")?.includes("hittable_session=")) {
        setCookie = clearSessionCookie(req);
      }
    }
    if (setCookie) res.headers.append("Set-Cookie", setCookie);
    return res;
  };
}

const NAME = /^[A-Za-z0-9_.-]{1,100}$/;

// "/repos/{owner}/{repo}" after checking both against GitHub's charset.
export function repoPath(p: { owner: string; repo: string }): string {
  for (const v of [p.owner, p.repo]) {
    if (!NAME.test(v) || v === "." || v === "..") throw new ApiError(400, "invalid", "Invalid owner or repository name");
  }
  return `/repos/${p.owner}/${p.repo}`;
}

export const isSha = (s: unknown): s is string => typeof s === "string" && /^[0-9a-f]{40}([0-9a-f]{24})?$/i.test(s);

// git check-ref-format, roughly.
export function isBranchName(s: unknown): s is string {
  return (
    typeof s === "string" &&
    s.length > 0 &&
    s.length <= 255 &&
    !/[\x00-\x20\x7f~^:?*[\\]/.test(s) &&
    !/(^|\/)\.|\.\.|\/\/|@\{|\.lock(\/|$)|\/$|\.$/.test(s) &&
    !s.startsWith("/") &&
    s !== "@"
  );
}

// A repo-relative path: no leading "/", no "." / ".." / empty segments.
export function isRepoPath(s: unknown): s is string {
  return (
    typeof s === "string" &&
    s.length > 0 &&
    s.length <= 4096 &&
    !s.includes("\\") &&
    !s.includes("\0") &&
    s.split("/").every((seg) => seg !== "" && seg !== "." && seg !== "..")
  );
}

// Each segment encoded on its own, so "feature/x" stays a path.
export const encodePath = (s: string) => s.split("/").map(encodeURIComponent).join("/");

export async function readJson<T>(req: Request, maxBytes = 64 * 1024): Promise<T> {
  const declared = Number(req.headers.get("content-length"));
  if (Number.isFinite(declared) && declared > maxBytes) throw new ApiError(413, "too_large", `Request is larger than ${maxBytes} bytes`);
  const text = await req.text();
  if (Buffer.byteLength(text) > maxBytes) throw new ApiError(413, "too_large", `Request is larger than ${maxBytes} bytes`);
  try {
    const v = JSON.parse(text);
    if (v && typeof v === "object" && !Array.isArray(v)) return v as T;
  } catch {
    // fall through
  }
  throw new ApiError(400, "invalid", "Expected a JSON object body");
}
