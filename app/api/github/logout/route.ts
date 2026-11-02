import { ApiError, errorResponse, json } from "@/utils/server/github/client";
import { githubConfig } from "@/utils/server/github/config";
import { assertSameOrigin } from "@/utils/server/github/guard";
import { SESSION_COOKIE, clearSessionCookie, readCookie, unseal, type Session } from "@/utils/server/github/session";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// Revokes the grant on GitHub (best effort), then clears the cookie. Signed out
// already is still {ok: true}. No refresh here: an expired token is just cleared.
export async function POST(req: Request) {
  let cfg;
  try {
    assertSameOrigin(req);
    cfg = githubConfig();
  } catch (err) {
    return errorResponse(err as ApiError);
  }
  const session = unseal<Session>(readCookie(req, SESSION_COOKIE), cfg.sessionSecret, SESSION_COOKIE);
  if (session?.t) {
    try {
      await fetch(`${cfg.apiUrl}/applications/${encodeURIComponent(cfg.clientId)}/grant`, {
        method: "DELETE",
        headers: {
          Accept: "application/vnd.github+json",
          "X-GitHub-Api-Version": "2022-11-28",
          "User-Agent": "hittable",
          "Content-Type": "application/json",
          Authorization: `Basic ${Buffer.from(`${cfg.clientId}:${cfg.clientSecret}`).toString("base64")}`,
        },
        body: JSON.stringify({ access_token: session.t }),
        cache: "no-store",
      });
    } catch {
      // ignored: the cookie is cleared either way
    }
  }
  const res = json({ ok: true });
  res.headers.append("Set-Cookie", clearSessionCookie(req));
  return res;
}
