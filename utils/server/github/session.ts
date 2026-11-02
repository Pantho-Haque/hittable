import { createCipheriv, createDecipheriv, createHash, randomBytes } from "node:crypto";
import type { GithubConfig } from "./config";
import { tokenRequest, type TokenSet } from "./client";

export const SESSION_COOKIE = "hittable_session";
const THIRTY_DAYS = 30 * 24 * 60 * 60;
const REFRESH_WINDOW = 5 * 60;

export type SessionUser = { login: string; name: string | null; avatarUrl: string; id: number };
export type Session = TokenSet & { u: SessionUser; v: 1 };

// AES-256-GCM, key = sha256(secret). Value = base64url(iv | tag | ciphertext).
// `purpose` is bound in as AAD, so one cookie cannot be replayed as another.
export function seal(data: unknown, secret: string, purpose: string): string {
  const iv = randomBytes(12);
  const cipher = createCipheriv("aes-256-gcm", createHash("sha256").update(secret).digest(), iv);
  cipher.setAAD(Buffer.from(purpose));
  const ct = Buffer.concat([cipher.update(JSON.stringify(data), "utf8"), cipher.final()]);
  return Buffer.concat([iv, cipher.getAuthTag(), ct]).toString("base64url");
}

export function unseal<T>(value: string | undefined, secret: string, purpose: string): T | null {
  if (!value) return null;
  try {
    const buf = Buffer.from(value, "base64url");
    if (buf.length < 29) return null;
    const decipher = createDecipheriv("aes-256-gcm", createHash("sha256").update(secret).digest(), buf.subarray(0, 12));
    decipher.setAAD(Buffer.from(purpose));
    decipher.setAuthTag(buf.subarray(12, 28));
    return JSON.parse(Buffer.concat([decipher.update(buf.subarray(28)), decipher.final()]).toString("utf8")) as T;
  } catch {
    return null;
  }
}

export function readCookie(req: Request, name: string): string | undefined {
  for (const part of (req.headers.get("cookie") ?? "").split(";")) {
    const eq = part.indexOf("=");
    if (eq > 0 && part.slice(0, eq).trim() === name) return part.slice(eq + 1).trim();
  }
  return undefined;
}

// Secure everywhere except plain-http localhost (dev and the e2e suite).
function isSecure(req: Request): boolean {
  const u = new URL(req.url);
  return !(u.protocol === "http:" && ["localhost", "127.0.0.1", "[::1]"].includes(u.hostname));
}

export function serializeCookie(req: Request, name: string, value: string, maxAge: number, path = "/"): string {
  return `${name}=${value}; Path=${path}; Max-Age=${Math.max(0, Math.floor(maxAge))}; HttpOnly; SameSite=Lax${isSecure(req) ? "; Secure" : ""}`;
}

export function sessionCookie(req: Request, cfg: GithubConfig, s: Session): string {
  // A refresh token outlives the access token, so keep the cookie for 30 days then;
  // an expiring token without one is useless after exp.
  const maxAge = s.exp && !s.rt ? s.exp - Math.floor(Date.now() / 1000) : THIRTY_DAYS;
  return serializeCookie(req, SESSION_COOKIE, seal(s, cfg.sessionSecret, SESSION_COOKIE), maxAge);
}

export const clearSessionCookie = (req: Request) => serializeCookie(req, SESSION_COOKIE, "", 0);

function isSession(s: unknown): s is Session {
  const v = s as Session | null;
  return !!v && v.v === 1 && typeof v.t === "string" && !!v.u && typeof v.u.login === "string";
}

// Refresh tokens are single-use, and the UI sends requests in parallel: every
// request carrying the same refresh token within a minute shares one refresh.
// ponytail: per instance; two instances racing on one token means the loser
// keeps the current token (still valid inside the window) and retries next time.
const refreshes = new Map<string, { at: number; p: Promise<TokenSet | null> }>();
function refreshOnce(cfg: GithubConfig, rt: string): Promise<TokenSet | null> {
  const now = Date.now();
  for (const [k, v] of refreshes) if (now - v.at > 60_000) refreshes.delete(k);
  let entry = refreshes.get(rt);
  if (!entry) {
    entry = { at: now, p: tokenRequest(cfg, { grant_type: "refresh_token", refresh_token: rt }) };
    refreshes.set(rt, entry);
  }
  return entry.p;
}

// Reads the session, refreshing an expiring token (one attempt) when it is within
// five minutes of exp. A failed refresh signs out once the token has expired;
// before that the current token is used. `setCookie`, when present, must go on
// the response: the refreshed session, or a clear.
export async function loadSession(req: Request, cfg: GithubConfig): Promise<{ session: Session | null; setCookie?: string }> {
  const raw = readCookie(req, SESSION_COOKIE);
  if (!raw) return { session: null };
  const s = unseal<Session>(raw, cfg.sessionSecret, SESSION_COOKIE);
  if (!isSession(s)) return { session: null, setCookie: clearSessionCookie(req) };
  const now = Math.floor(Date.now() / 1000);
  if (!s.exp || s.exp - now > REFRESH_WINDOW) return { session: s };
  const fresh = s.rt ? await refreshOnce(cfg, s.rt) : null;
  if (!fresh) return s.exp > now ? { session: s } : { session: null, setCookie: clearSessionCookie(req) };
  const next: Session = { ...fresh, rt: fresh.rt ?? s.rt, u: s.u, v: 1 };
  return { session: next, setCookie: sessionCookie(req, cfg, next) };
}
