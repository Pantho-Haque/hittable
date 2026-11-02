import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SESSION_COOKIE, loadSession, seal, unseal, type Session } from "@/utils/server/github/session";
import { githubConfig } from "@/utils/server/github/config";
import { GET as reposGET } from "@/app/api/github/repos/route";
import { GET as sessionGET } from "@/app/api/github/session/route";
import { API, OAUTH, SECRET, cookieValue, ghJson, mockFetch, params, req, sessionValue, setCookie, stubEnv, user } from "./helpers";

beforeEach(stubEnv);
afterEach(() => vi.unstubAllEnvs());

describe("session crypto", () => {
  it("round-trips and hides the plaintext", () => {
    const v = seal({ t: "gho_secret", u: user, v: 1 }, SECRET, SESSION_COOKIE);
    expect(v).not.toContain("gho_secret");
    expect(Buffer.from(v, "base64url").toString("latin1")).not.toContain("gho_secret");
    expect(unseal<Session>(v, SECRET, SESSION_COOKIE)?.t).toBe("gho_secret");
  });

  it("rejects a tampered value", () => {
    const buf = Buffer.from(seal({ a: 1 }, SECRET, SESSION_COOKIE), "base64url");
    buf[buf.length - 1] ^= 1;
    expect(unseal(buf.toString("base64url"), SECRET, SESSION_COOKIE)).toBeNull();
    expect(unseal("garbage", SECRET, SESSION_COOKIE)).toBeNull();
    expect(unseal("", SECRET, SESSION_COOKIE)).toBeNull();
  });

  it("rejects the wrong key and the wrong purpose", () => {
    const v = seal({ a: 1 }, SECRET, SESSION_COOKIE);
    expect(unseal(v, "x".repeat(40), SESSION_COOKIE)).toBeNull();
    expect(unseal(v, SECRET, "hittable_oauth")).toBeNull();
  });
});

describe("config", () => {
  it("refuses a missing or short SESSION_SECRET", () => {
    vi.stubEnv("SESSION_SECRET", "short");
    expect(() => githubConfig()).toThrow(/at least 32 bytes/);
    vi.stubEnv("SESSION_SECRET", "");
    expect(() => githubConfig()).toThrow(/SESSION_SECRET/);
  });

  it("defaults the GraphQL URL from the API URL", () => {
    expect(githubConfig().graphqlUrl).toBe(`${API}/graphql`);
  });
});

describe("session route", () => {
  it("returns the user, or null without 401", async () => {
    const res = await sessionGET(req("/api/github/session"));
    expect(await res.json()).toEqual({ user: { login: "octo", name: "Octo Cat", avatarUrl: user.avatarUrl } });
    const anon = await sessionGET(req("/api/github/session", { session: null }));
    expect(anon.status).toBe(200);
    expect(await anon.json()).toEqual({ user: null });
  });

  it("clears an unreadable cookie", async () => {
    const res = await sessionGET(req("/api/github/session", { session: "bogus" }));
    expect(await res.json()).toEqual({ user: null });
    expect(setCookie(res, SESSION_COOKIE)).toMatch(/Max-Age=0/);
  });
});

describe("refresh", () => {
  const now = () => Math.floor(Date.now() / 1000);

  it("does not refresh a non-expiring token", async () => {
    const calls = mockFetch(() => ghJson({}));
    const r = await loadSession(req("/x"), githubConfig());
    expect(r.session?.t).toBe("tok");
    expect(r.setCookie).toBeUndefined();
    expect(calls).toHaveLength(0);
  });

  it("refreshes a token near expiry and rewrites the cookie", async () => {
    const calls = mockFetch((url) => {
      if (url.href === `${OAUTH}/login/oauth/access_token`) {
        return ghJson({ access_token: "new-tok", refresh_token: "new-rt", expires_in: 28800 });
      }
      return ghJson([{ full_name: "o/r", name: "r", owner: { login: "o" }, permissions: { push: true } }]);
    });
    const res = await reposGET(
      req("/api/github/repos", { session: sessionValue({ rt: "rt-ok", exp: now() + 60 }) }),
      params({}),
    );
    expect(res.status).toBe(200);
    const body = new URLSearchParams(calls[0].body);
    expect(body.get("grant_type")).toBe("refresh_token");
    expect(body.get("refresh_token")).toBe("rt-ok");
    expect(calls[1].headers.get("authorization")).toBe("Bearer new-tok");
    const fresh = unseal<Session>(cookieValue(setCookie(res, SESSION_COOKIE)), SECRET, SESSION_COOKIE)!;
    expect(fresh).toMatchObject({ t: "new-tok", rt: "new-rt", u: { login: "octo" } });
    expect(fresh.exp).toBeGreaterThan(now() + 28000);
  });

  it("clears the session and answers 401 when the refresh fails after expiry", async () => {
    const calls = mockFetch(() => ghJson({ error: "bad_refresh_token" }));
    const res = await reposGET(
      req("/api/github/repos", { session: sessionValue({ rt: "rt-fail", exp: now() - 10 }) }),
      params({}),
    );
    expect(res.status).toBe(401);
    expect(await res.json()).toEqual({ error: "signed_out" });
    expect(setCookie(res, SESSION_COOKIE)).toMatch(/Max-Age=0/);
    expect(calls).toHaveLength(1); // single attempt, no API call
  });

  it("keeps the current token when a refresh fails before expiry", async () => {
    const calls = mockFetch((url) => (url.origin === OAUTH ? ghJson({ error: "bad_refresh_token" }) : ghJson([])));
    const res = await reposGET(
      req("/api/github/repos", { session: sessionValue({ rt: "rt-race", exp: now() + 60 }) }),
      params({}),
    );
    expect(res.status).toBe(200);
    expect(setCookie(res, SESSION_COOKIE)).toBeUndefined();
    expect(calls[1].headers.get("authorization")).toBe("Bearer tok");
  });

  it("shares one refresh between parallel requests", async () => {
    const calls = mockFetch((url) =>
      url.origin === OAUTH ? ghJson({ access_token: "par-tok", refresh_token: "par-rt2", expires_in: 28800 }) : ghJson([]),
    );
    const cookie = sessionValue({ rt: "rt-par", exp: now() + 60 });
    const [a, b] = await Promise.all([1, 2].map(() => reposGET(req("/api/github/repos", { session: cookie }), params({}))));
    expect(calls.filter((c) => c.url.startsWith(OAUTH))).toHaveLength(1);
    for (const r of [a, b]) {
      expect(unseal<Session>(cookieValue(setCookie(r, SESSION_COOKIE)), SECRET, SESSION_COOKIE)?.t).toBe("par-tok");
    }
  });
});
