// Small per-device memories for the GitHub workspace. Storage can be missing
// or throw (private mode, blocked site data), so every access is guarded.

const RECENT_KEY = "hittable_recent_repos";
const HINT_KEY = "hittable_gh_signed_in";
const MAX_RECENT = 5;

/** Full names ("owner/repo"), most recent first. */
export function readRecent(): string[] {
  try {
    const list = JSON.parse(localStorage.getItem(RECENT_KEY) ?? "[]");
    return Array.isArray(list) ? list.filter((x): x is string => typeof x === "string") : [];
  } catch {
    return [];
  }
}

export function rememberRecent(fullName: string) {
  try {
    const next = [fullName, ...readRecent().filter((n) => n.toLowerCase() !== fullName.toLowerCase())].slice(0, MAX_RECENT);
    localStorage.setItem(RECENT_KEY, JSON.stringify(next));
  } catch {
    // not remembered; harmless
  }
}

/** Whether this device was signed in last time: shows the picker's skeleton instead of a sign-in flash. */
export function signedInHint(): boolean {
  try {
    return localStorage.getItem(HINT_KEY) === "1";
  } catch {
    return false;
  }
}

export function setSignedInHint(on: boolean) {
  try {
    if (on) localStorage.setItem(HINT_KEY, "1");
    else localStorage.removeItem(HINT_KEY);
  } catch {
    // ignored
  }
}
