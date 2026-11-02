import { createHash, randomBytes, timingSafeEqual } from "node:crypto";
import { appOrigin, githubConfig, type GithubConfig } from "./config";
import { ApiError, errorResponse, ghJson, tokenRequest } from "./client";
import { readCookie, seal, serializeCookie, sessionCookie, unseal, type Session } from "./session";

export const OAUTH_COOKIE = "hittable_oauth";
const OAUTH_TTL = 10 * 60;
const OAUTH_PATH = "/api/github";
export const SCOPE = "repo read:user";
export const DEFAULT_NEXT = "/hittable";

type Pending = { state: string; verifier: string; next: string; iat: number };

export const randomToken = () => randomBytes(32).toString("base64url");

// RFC 7636 S256.
export const pkceChallenge = (verifier: string) => createHash("sha256").update(verifier).digest("base64url");

// Only a same-origin path: "/x", never "//host", "/\host" or an absolute URL.
export function safeNext(next: string | null | undefined): string {
  if (!next || !next.startsWith("/") || next.startsWith("//") || next.includes("\\")) return DEFAULT_NEXT;
  try {
    const u = new URL(next, "http://n.invalid");
    return u.origin === "http://n.invalid" ? u.pathname + u.search + u.hash : DEFAULT_NEXT;
  } catch {
    return DEFAULT_NEXT;
  }
}

export function safeEqual(a: string, b: string): boolean {
  const x = Buffer.from(a);
  const y = Buffer.from(b);
  return x.length === y.length && timingSafeEqual(x, y);
}

const redirectUri = (req: Request, cfg: GithubConfig) => `${appOrigin(req, cfg)}/api/github/callback`;

function redirect(location: string, cookies: string[]): Response {
  const headers = new Headers({ Location: location, "Cache-Control": "no-store" });
  for (const c of cookies) headers.append("Set-Cookie", c);
  return new Response(null, { status: 302, headers });
}

// GET /api/github/login?next=/path
export function startLogin(req: Request): Response {
  let cfg: GithubConfig;
  try {
    cfg = githubConfig();
  } catch (err) {
    return errorResponse(err as ApiError);
  }
  const pending: Pending = {
    state: randomToken(),
    verifier: randomToken(),
    next: safeNext(new URL(req.url).searchParams.get("next")),
    iat: Math.floor(Date.now() / 1000),
  };
  const auth = new URL(`${cfg.oauthUrl}/login/oauth/authorize`);
  auth.search = new URLSearchParams({
    client_id: cfg.clientId,
    redirect_uri: redirectUri(req, cfg),
    scope: SCOPE,
    state: pending.state,
    code_challenge: pkceChallenge(pending.verifier),
    code_challenge_method: "S256",
    allow_signup: "true",
  }).toString();
  return redirect(auth.toString(), [
    serializeCookie(req, OAUTH_COOKIE, seal(pending, cfg.sessionSecret, OAUTH_COOKIE), OAUTH_TTL, OAUTH_PATH),
  ]);
}

type GhUser = { login: string; id: number; name: string | null; avatar_url: string };

// GET /api/github/callback?code&state
export async function finishLogin(req: Request): Promise<Response> {
  const clearTemp = serializeCookie(req, OAUTH_COOKIE, "", 0, OAUTH_PATH);
  let cfg: GithubConfig;
  try {
    cfg = githubConfig();
  } catch {
    return redirect(`${new URL(req.url).origin}${DEFAULT_NEXT}?auth_error=config`, [clearTemp]);
  }
  const origin = appOrigin(req, cfg);
  const fail = (code: string) => redirect(`${origin}${DEFAULT_NEXT}?auth_error=${code}`, [clearTemp]);
  const q = new URL(req.url).searchParams;

  if (q.get("error")) return fail(q.get("error") === "access_denied" ? "denied" : "exchange_failed");
  const pending = unseal<Pending>(readCookie(req, OAUTH_COOKIE), cfg.sessionSecret, OAUTH_COOKIE);
  if (!pending?.verifier || !pending.state || Math.floor(Date.now() / 1000) - pending.iat > OAUTH_TTL) {
    return fail("missing_verifier");
  }
  if (!safeEqual(q.get("state") ?? "", pending.state)) return fail("state_mismatch");
  const code = q.get("code");
  if (!code) return fail("exchange_failed");

  const tokens = await tokenRequest(cfg, { code, redirect_uri: redirectUri(req, cfg), code_verifier: pending.verifier });
  if (!tokens) return fail("exchange_failed");
  let user: GhUser;
  try {
    user = await ghJson<GhUser>({ cfg, token: tokens.t }, "/user");
  } catch {
    return fail("exchange_failed");
  }
  const session: Session = {
    ...tokens,
    u: { login: user.login, name: user.name ?? null, avatarUrl: user.avatar_url, id: user.id },
    v: 1,
  };
  return redirect(`${origin}${safeNext(pending.next)}`, [sessionCookie(req, cfg, session), clearTemp]);
}
