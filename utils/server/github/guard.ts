import { ApiError } from "./client";

// CSRF guard for mutating routes. A cross-site page cannot send X-Hittable
// without a CORS preflight (which we never answer), and the Origin must be this host.
export function assertSameOrigin(req: Request): void {
  if (req.headers.get("x-hittable") !== "1") {
    throw new ApiError(403, "bad_origin", "Missing X-Hittable header");
  }
  let originHost: string;
  try {
    originHost = new URL(req.headers.get("origin") ?? "").host;
  } catch {
    throw new ApiError(403, "bad_origin", "Missing or invalid Origin");
  }
  const hosts = [req.headers.get("host"), req.headers.get("x-forwarded-host"), new URL(req.url).host];
  if (!hosts.includes(originHost)) throw new ApiError(403, "bad_origin", "Cross-origin request refused");
}
