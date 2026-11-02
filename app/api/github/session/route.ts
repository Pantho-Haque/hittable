import { ApiError, errorResponse, json } from "@/utils/server/github/client";
import { githubConfig } from "@/utils/server/github/config";
import { loadSession } from "@/utils/server/github/session";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// Never 401: signed out is {user: null}.
export async function GET(req: Request) {
  let cfg;
  try {
    cfg = githubConfig();
  } catch (err) {
    return errorResponse(err as ApiError);
  }
  const { session, setCookie } = await loadSession(req, cfg);
  const res = json({ user: session ? { login: session.u.login, name: session.u.name, avatarUrl: session.u.avatarUrl } : null });
  if (setCookie) res.headers.append("Set-Cookie", setCookie);
  return res;
}
