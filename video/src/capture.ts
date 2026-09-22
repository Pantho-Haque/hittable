import raw from "../public/capture.json";

/** [text, fg, bg, attrs] — arrays, because a capture holds tens of thousands. */
export type Run = [string, string | number | null, string | number | null, number];

export type Scene = {
  name: string;
  caption: string;
  note?: string;
  focus: [number, number, number, number]; // x, y, w, h in cells
  startMs: number;
  endMs: number;
};

type Frame = { ms: number; rows: Record<string, Run[]> };
type Capture = { cols: number; rows: number; scenes: Scene[]; frames: Frame[] };

const capture = raw as unknown as Capture;

export const COLS = capture.cols;
export const ROWS = capture.rows;
export const scenes = capture.scenes;
export const durationMs = scenes[scenes.length - 1].endMs;

/** attr bits, mirroring the recorder. */
export const BOLD = 1 << 2;
export const UNDERLINE = 1 << 1;
export const ITALIC = 1 << 4;

/**
 * The capture stores only the rows that changed, so the full screen at a given
 * moment is every delta up to it, applied in order. Accumulating once here
 * turns that into a flat list the player can binary-search.
 */
const snapshots: { ms: number; grid: Run[][] }[] = (() => {
  const grid: Run[][] = Array.from({ length: ROWS }, () => []);
  return capture.frames.map((f) => {
    for (const [y, runs] of Object.entries(f.rows)) grid[Number(y)] = runs;
    return { ms: f.ms, grid: grid.map((r) => r) };
  });
})();

export function gridAt(ms: number): Run[][] {
  let lo = 0;
  let hi = snapshots.length - 1;
  let found = 0;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    if (snapshots[mid].ms <= ms) {
      found = mid;
      lo = mid + 1;
    } else hi = mid - 1;
  }
  return snapshots[found].grid;
}

export function sceneAt(ms: number): Scene {
  for (const s of scenes) if (ms >= s.startMs && ms < s.endMs) return s;
  return scenes[scenes.length - 1];
}

// ---- colour ----

export const DEFAULT_FG = "#f8f8f2";
export const DEFAULT_BG = "#282a36";

/** The first 16 slots match the app's Dracula palette; 16-255 are xterm's. */
const ANSI16 = [
  "#21222c", "#ff5555", "#50fa7b", "#f1fa8c", "#bd93f9", "#ff79c6", "#8be9fd", "#f8f8f2",
  "#6272a4", "#ff6e6e", "#69ff94", "#ffffa5", "#d6acff", "#ff92df", "#a4ffff", "#ffffff",
];

const palette: string[] = (() => {
  const out = [...ANSI16];
  const level = [0, 95, 135, 175, 215, 255];
  for (let r = 0; r < 6; r++)
    for (let g = 0; g < 6; g++)
      for (let b = 0; b < 6; b++)
        out.push(`rgb(${level[r]},${level[g]},${level[b]})`);
  for (let i = 0; i < 24; i++) {
    const v = 8 + i * 10;
    out.push(`rgb(${v},${v},${v})`);
  }
  return out;
})();

export function colour(c: string | number | null, fallback: string): string {
  if (c === null || c === undefined) return fallback;
  if (typeof c === "number") return palette[c] ?? fallback;
  return c;
}
