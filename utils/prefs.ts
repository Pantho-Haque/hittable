import { useState } from "react";

// Per-browser conveniences (a layout, a toggle). Storage can be missing or
// blocked, in which case the default applies and nothing is remembered.
export function readPref(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function writePref(key: string, value: string) {
  try {
    localStorage.setItem(key, value);
  } catch {
    // not remembered; still applies now
  }
}

/** A boolean remembered under `key`. Client-only components: read on first render. */
export function useStoredFlag(key: string, fallback: boolean): [boolean, (next: boolean) => void] {
  const [value, setValue] = useState(() => {
    const saved = readPref(key);
    return saved === null ? fallback : saved === "1";
  });
  return [
    value,
    (next) => {
      setValue(next);
      writePref(key, next ? "1" : "0");
    },
  ];
}
