import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { OAUTH_COOKIE, pkceChallenge, safeNext } from "@/utils/server/github/oauth";
import { SESSION_COOKIE, seal, unseal, type Session } from "@/utils/server/github/session";
import { GET as login } from "@/app/api/github/login/route";
import { GET as callback } from "@/app/api/github/callback/route";
import { API, APP, OAUTH, SECRET, cookieValue, ghJson, mockFetch, setCookie, stubEnv } from "./helpers";

beforeEach(stubEnv);
afterEach(() => vi.unstubAllEnvs());

it("computes the RFC 7636 appendix B S256 challenge", () => {
  expect(pkceChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")).toBe("E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM");
});

it.each([
  ["/hittable/gh/o/r?ref=x#y", "/hittable/gh/o/r?ref=x#y"],
  ["//evil.com", "/hittable"],
  ["/\\evil.com", "/hittable"],
  ["https://evil", "/hittable"],
  ["evil", "/hittable"],
  ["", "/hittable"],
  [null, "/hittable"],
])("safeNext(%s) = %s", (input, out) => {
  expect(safeNext(input)).toBe(out);
});

describe("login", () => {
  it("redirects to authorize with state, S256 PKCE and the scope", async () => {
    const res = login(new Request(`${APP}/api/github/login?next=/hittable/gh/o/r`));
    expect(res.status).toBe(302);
    const loc = new URL(res.headers.get("location")!);
    expect(loc.origin + loc.pathname).toBe(`${OAUTH}/login/oauth/authorize`);
    const q = loc.searchParams;
    expect(q.get("client_id")).toBe("cid");
    expect(q.get("redirect_uri")).toBe(`${APP}/api/github/callback`);
    expect(q.get("scope")).toBe("repo read:user");
    expect(q.get("code_challenge_method")).toBe("S256");
    expect(q.get("allow_signup")).toBe("true");
    const line = setCookie(res, OAUTH_COOKIE)!;
    expect(line).toMatch(/HttpOnly/);
    expect(line).toMatch(/SameSite=Lax/);
    expect(line).toMatch(/Secure/);
    expect(line).toMatch(/Max-Age=600/);
    const pending = unseal<{ state: string; verifier: string; next: string }>(cookieValue(line), SECRET, OAUTH_COOKIE)!;
    expect(pending.state).toBe(q.get("state"));
    expect(pkceChallenge(pending.verifier)).toBe(q.get("code_challenge"));
    expect(pending.next).toBe("/hittable/gh/o/r");
  });

  it("sanitises next and uses APP_URL", async () => {
    vi.stubEnv("APP_URL", "https://public.test/");
    const res = login(new Request(`${APP}/api/github/login?next=//evil.com`));
    const loc = new URL(res.headers.get("location")!);
    expect(loc.searchParams.get("redirect_uri")).toBe("https://public.test/api/github/callback");
    const pending = unseal<{ next: string }>(cookieValue(setCookie(res, OAUTH_COOKIE)), SECRET, OAUTH_COOKIE)!;
    expect(pending.next).toBe("/hittable");
  });

  it("leaves Secure off for plain-http localhost", () => {
    expect(setCookie(login(new Request("http://localhost:3000/api/github/login")), OAUTH_COOKIE)).not.toMatch(/Secure/);
  });

  it("refuses with a 500 JSON error when not configured", async () => {
    vi.stubEnv("SESSION_SECRET", "too-short");
    const res = login(new Request(`${APP}/api/github/login`));
    expect(res.status).toBe(500);
    expect(await res.json()).toMatchObject({ error: "config" });
  });
});

describe("callback", () => {
  const pending = (over = {}) =>
    seal({ state: "st", verifier: "ver", next: "/hittable/gh/o/r", iat: Math.floor(Date.now() / 1000), ...over }, SECRET, OAUTH_COOKIE);
  const cb = (query: string, cookie: string | null = pending()) =>
    callback(new Request(`${APP}/api/github/callback?${query}`, { headers: cookie ? { cookie: `${OAUTH_COOKIE}=${cookie}` } : {} }));
  const authError = (res: Response) => {
    expect(res.status).toBe(302);
    const loc = new URL(res.headers.get("location")!);
    expect(loc.pathname).toBe("/hittable");
    expect(setCookie(res, OAUTH_COOKIE)).toMatch(/Max-Age=0/);
    return loc.searchParams.get("auth_error");
  };

  it("rejects a forged state", async () => {
    const calls = mockFetch(() => ghJson({}));
    expect(authError(await cb("code=c&state=forged"))).toBe("state_mismatch");
    expect(calls).toHaveLength(0);
  });

  it("rejects a callback without the PKCE cookie", async () => {
    expect(authError(await cb("code=c&state=st", null))).toBe("missing_verifier");
    expect(authError(await cb("code=c&state=st", "tampered"))).toBe("missing_verifier");
    expect(authError(await cb("code=c&state=st", pending({ iat: 1 })))).toBe("missing_verifier");
  });

  it("reports a denied authorization", async () => {
    expect(authError(await cb("error=access_denied&state=st"))).toBe("denied");
  });

  it("reports a failed exchange", async () => {
    mockFetch(() => ghJson({ error: "bad_verification_code" }));
    expect(authError(await cb("code=c&state=st"))).toBe("exchange_failed");
  });

  it("reports missing config", async () => {
    vi.stubEnv("GITHUB_CLIENT_SECRET", "");
    expect(authError(await cb("code=c&state=st"))).toBe("config");
  });

  it("exchanges the code with the verifier, sets the session and returns to next", async () => {
    const calls = mockFetch((url) =>
      url.href === `${OAUTH}/login/oauth/access_token`
        ? ghJson({ access_token: "gho_tok", token_type: "bearer" })
        : ghJson({ login: "octo", id: 7, name: "Octo", avatar_url: "https://a.test/7" }),
    );
    const res = await cb("code=thecode&state=st");
    expect(res.status).toBe(302);
    expect(res.headers.get("location")).toBe(`${APP}/hittable/gh/o/r`);
    expect(res.headers.get("location")).not.toContain("gho_tok");

    const ex = new URLSearchParams(calls[0].body);
    expect(Object.fromEntries(ex)).toEqual({
      client_id: "cid",
      client_secret: "csecret",
      code: "thecode",
      redirect_uri: `${APP}/api/github/callback`,
      code_verifier: "ver",
    });
    expect(calls[0].headers.get("accept")).toBe("application/json");
    expect(calls[1].url).toBe(`${API}/user`);
    expect(calls[1].headers.get("authorization")).toBe("Bearer gho_tok");

    const line = setCookie(res, SESSION_COOKIE)!;
    expect(line).toMatch(/HttpOnly/);
    expect(line).toMatch(/Path=\//);
    expect(line).not.toContain("gho_tok");
    expect(unseal<Session>(cookieValue(line), SECRET, SESSION_COOKIE)).toMatchObject({
      t: "gho_tok",
      v: 1,
      u: { login: "octo", id: 7, name: "Octo", avatarUrl: "https://a.test/7" },
    });
    expect(setCookie(res, OAUTH_COOKIE)).toMatch(/Max-Age=0/);
  });
});
