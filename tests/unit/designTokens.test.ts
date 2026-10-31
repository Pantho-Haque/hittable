import { describe, it, expect } from "vitest";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

// NFR-1 / NFR-2: components draw colours from the tokens in styles/theme.css
// and never set text below 11px. These used to drift: ~50 hard-coded hex
// values and 7–10px labels throughout the workspace.

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) return sources(p);
    return /\.tsx$/.test(name) ? [p] : [];
  });
}

const files = [...sources("components"), ...sources("app")];

describe("design tokens", () => {
  it("no hex colour in Tailwind arbitrary classes", () => {
    const offenders = files.flatMap((f) =>
      [...readFileSync(f, "utf8").matchAll(/-\[#[0-9a-fA-F]{3,8}\]/g)].map((m) => `${f}: ${m[0]}`),
    );
    expect(offenders).toEqual([]);
  });

  it("the accent colour comes from --brand, not a literal", () => {
    const offenders = files.filter((f) => /["'`]#00e5cc["'`]/i.test(readFileSync(f, "utf8")));
    expect(offenders).toEqual([]);
  });

  it("no text smaller than 11px", () => {
    const offenders = files.flatMap((f) =>
      [...readFileSync(f, "utf8").matchAll(/text-\[(\d+(?:\.\d+)?)px\]/g)]
        .filter((m) => Number(m[1]) < 11)
        .map((m) => `${f}: ${m[0]}`),
    );
    expect(offenders).toEqual([]);
  });
});
