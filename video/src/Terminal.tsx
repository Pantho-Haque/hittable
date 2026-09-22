import React from "react";
import {
  BOLD, COLS, DEFAULT_BG, DEFAULT_FG, ITALIC, ROWS, Run, UNDERLINE, colour, gridAt,
} from "./capture";

/**
 * Cell metrics. The grid is laid out by natural monospace advance rather than
 * positioning every cell, which keeps a frame to a few hundred spans instead of
 * ~4,700 — the difference between a render that finishes and one that crawls.
 * CELL_W must match the font's advance ratio or the zoom rectangles drift.
 */
export const FONT_SIZE = 16;
export const CELL_W = FONT_SIZE * 0.6;
export const CELL_H = 20;
export const TERM_W = COLS * CELL_W;
export const TERM_H = ROWS * CELL_H;

export const MONO =
  '"SF Mono", SFMono-Regular, Menlo, Monaco, "DejaVu Sans Mono", "Courier New", monospace';

const Row: React.FC<{ runs: Run[] }> = ({ runs }) => (
  <div style={{ height: CELL_H, whiteSpace: "pre" }}>
    {runs.map(([text, fg, bg, attr], i) => (
      <span
        key={i}
        style={{
          color: colour(fg, DEFAULT_FG),
          background: bg === null ? undefined : colour(bg, DEFAULT_BG),
          fontWeight: attr & BOLD ? 700 : 400,
          fontStyle: attr & ITALIC ? "italic" : undefined,
          textDecoration: attr & UNDERLINE ? "underline" : undefined,
        }}
      >
        {text}
      </span>
    ))}
  </div>
);

export const Terminal: React.FC<{ ms: number }> = ({ ms }) => {
  const grid = gridAt(ms);
  return (
    <div
      style={{
        width: TERM_W,
        height: TERM_H,
        background: DEFAULT_BG,
        fontFamily: MONO,
        fontSize: FONT_SIZE,
        lineHeight: `${CELL_H}px`,
        fontVariantLigatures: "none",
        letterSpacing: 0,
        borderRadius: 10,
        overflow: "hidden",
      }}
    >
      {grid.map((runs, y) => (
        <Row key={y} runs={runs} />
      ))}
    </div>
  );
};
