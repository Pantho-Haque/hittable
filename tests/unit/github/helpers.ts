import { vi } from "vitest";
import { SESSION_COOKIE, seal, type Session } from "@/utils/server/github/session";

export const SECRET = "s".repeat(40);
export const APP = "https://app.test";
export const API = "https://api.gh.test";
export const OAUTH = "https://gh.test";

export function stubEnv() {
  vi.stubEnv("GITHUB_CLIENT_ID", "cid");
  vi.stubEnv("GITHUB_CLIENT_SECRET", "csecret");
  vi.stubEnv("SESSION_SECRET", SECRET);
  vi.stubEnv("GITHUB_API_URL", API);
  vi.stubEnv("GITHUB_OAUTH_URL", OAUTH);
  vi.stubEnv("GITHUB_GRAPHQL_URL", "");
  vi.stubEnv("APP_URL", "");
}

export const user = { login: "octo", name: "Octo Cat", avatarUrl: "https://avatars.test/1", id: 1 };

export function sessionValue(over: Partial<Session> = {}): string {
  return seal({ t: "tok", u: user, v: 1, ...over }, SECRET, SESSION_COOKIE);
}

export type Call = { url: string; method: string; headers: Headers; body: string };

// Replaces global fetch with a router; records every call.
export function mockFetch(route: (url: URL, call: Call) => Response | Promise<Response>) {
  const calls: Call[] = [];
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const call: Call = {
      url: String(input),
      method: init?.method ?? "GET",
      headers: new Headers(init?.headers),
      body: init?.body ? String(init.body) : "",
    };
    calls.push(call);
    return route(new URL(call.url), call);
  });
  return calls;
}

export function req(path: string, init: RequestInit & { session?: string | null; mutating?: boolean } = {}) {
  const { session = sessionValue(), mutating, ...rest } = init;
  const headers = new Headers(rest.headers);
  if (session) headers.set("cookie", `${SESSION_COOKIE}=${session}`);
  if (mutating) {
    headers.set("origin", APP);
    headers.set("x-hittable", "1");
  }
  return new Request(APP + path, { ...rest, headers });
}

export const params = <P>(p: P) => ({ params: Promise.resolve(p) });

export const ghJson = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
  Response.json(body, { status, headers });

// Value of the named cookie in a response's Set-Cookie lines.
export function setCookie(res: Response, name: string): string | undefined {
  return res.headers.getSetCookie().find((c) => c.startsWith(name + "="));
}
export const cookieValue = (line: string | undefined) => line?.split(";")[0].split("=").slice(1).join("=");
