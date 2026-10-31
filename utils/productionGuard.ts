// FR-16: an environment is production when it defines HITTABLE_PRODUCTION as
// true. It is an ordinary variable, so env.json, local collections, exports
// and the terminal app need nothing new to carry it.

export const PRODUCTION_VAR = "HITTABLE_PRODUCTION";

/** Methods that only read; everything else changes state and gets confirmed. */
const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);

export function isProduction(env?: Record<string, string>, secrets?: Record<string, string>): boolean {
  const v = (secrets?.[PRODUCTION_VAR] ?? env?.[PRODUCTION_VAR] ?? "").trim().toLowerCase();
  return v === "true" || v === "1" || v === "yes";
}

export function needsConfirmation(method: string, env?: Record<string, string>, secrets?: Record<string, string>): boolean {
  return isProduction(env, secrets) && !SAFE_METHODS.has(method.toUpperCase());
}

/** The host the request goes to, for the confirmation text (variables filled in). */
export function targetHost(url: string, env?: Record<string, string>, secrets?: Record<string, string>): string {
  const vars = { ...(env ?? {}), ...(secrets ?? {}) };
  const resolved = url.replace(/<<(\w+)>>/g, (m, k: string) => vars[k] ?? m);
  try {
    return new URL(resolved.includes("://") ? resolved : `http://${resolved}`).host;
  } catch {
    return resolved;
  }
}
