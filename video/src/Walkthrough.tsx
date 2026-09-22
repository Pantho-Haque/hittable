import React from "react";
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from "remotion";
import { COLS, ROWS, Scene, scenes } from "./capture";
import { CELL_H, CELL_W, MONO, TERM_H, TERM_W, Terminal } from "./Terminal";
import { BG, CYAN } from "./Brand";

const VW = 1920;
const VH = 1080;
// The caption band sits over the bottom of the frame, so the terminal is
// scaled and lifted to keep its footer clear of the text.
const CAPTION_BAND = 150;
const MAX_ZOOM = 2.6;

type View = { scale: number; x: number; y: number };

/**
 * viewFor turns a scene's focus rectangle into a transform. An empty rectangle
 * means "show everything"; otherwise the rectangle is fitted to the frame and
 * centred, which is what gives each beat its zoom.
 */
function viewFor(s: Scene): View {
  const usableH = VH - CAPTION_BAND;
  const whole = Math.min(VW / TERM_W, usableH / TERM_H) * 0.97;
  const lift = -CAPTION_BAND / 2;
  const [fx, fy] = s.focus;
  // A zero width or height means "out to the edge".
  const fw = s.focus[2] > 0 ? s.focus[2] : COLS - fx;
  const fh = s.focus[3] > 0 ? s.focus[3] : ROWS - fy;
  if (fw >= COLS && fh >= ROWS) return { scale: whole, x: 0, y: lift };

  const scale = Math.min(
    Math.max(Math.min((VW * 0.9) / (fw * CELL_W), (usableH * 0.92) / (fh * CELL_H)), whole),
    MAX_ZOOM,
  );
  // A rectangle that fills a whole axis cannot zoom, and panning at full size
  // would just push the terminal off the frame. Show everything instead.
  if (scale <= whole * 1.03) return { scale: whole, x: 0, y: lift };
  // Offset that brings the rectangle's centre to the middle of the frame.
  const cx = (fx + fw / 2) * CELL_W - TERM_W / 2;
  const cy = (fy + fh / 2) * CELL_H - TERM_H / 2;
  return { scale, x: -cx * scale, y: -cy * scale + lift };
}

const lerp = (a: number, b: number, t: number) => a + (b - a) * t;

const Caption: React.FC<{ scene: Scene; frame: number; fps: number }> = ({ scene, frame, fps }) => {
  const start = (scene.startMs / 1000) * fps;
  const end = (scene.endMs / 1000) * fps;
  const inn = spring({ frame: frame - start - 4, fps, config: { damping: 200 } });
  const out = interpolate(frame, [end - 10, end - 2], [1, 0], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });
  const o = Math.min(inn, out);

  return (
    <AbsoluteFill style={{ justifyContent: "flex-end", pointerEvents: "none" }}>
      <div
        style={{
          padding: "120px 96px 62px",
          background: "linear-gradient(to top, rgba(16,17,22,0.94) 22%, rgba(16,17,22,0) 100%)",
          opacity: o,
          transform: `translateY(${(1 - o) * 16}px)`,
        }}
      >
        <div style={{ fontFamily: MONO, fontSize: 46, fontWeight: 700, color: "#f2f3f7" }}>
          {scene.caption}
        </div>
        {scene.note ? (
          <div style={{ fontFamily: MONO, fontSize: 25, color: CYAN, marginTop: 14, opacity: 0.85 }}>
            {scene.note}
          </div>
        ) : null}
      </div>
    </AbsoluteFill>
  );
};

export const Walkthrough: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const ms = (frame / fps) * 1000;

  let i = scenes.findIndex((s) => ms >= s.startMs && ms < s.endMs);
  if (i < 0) i = scenes.length - 1;
  const scene = scenes[i];

  // Ease from the previous scene's framing into this one, so the camera moves
  // rather than cutting.
  const from = viewFor(scenes[Math.max(i - 1, 0)]);
  const to = viewFor(scene);
  const t = spring({
    frame: frame - (scene.startMs / 1000) * fps,
    fps,
    config: { damping: 200, mass: 1.1 },
  });
  const view = {
    scale: lerp(from.scale, to.scale, t),
    x: lerp(from.x, to.x, t),
    y: lerp(from.y, to.y, t),
  };

  return (
    <AbsoluteFill style={{ background: BG }}>
      <AbsoluteFill style={{ alignItems: "center", justifyContent: "center" }}>
        <div
          style={{
            transform: `translate(${view.x}px, ${view.y}px) scale(${view.scale})`,
            boxShadow: "0 40px 120px rgba(0,0,0,0.55)",
            borderRadius: 10,
          }}
        >
          <Terminal ms={ms} />
        </div>
      </AbsoluteFill>
      <Caption scene={scene} frame={frame} fps={fps} />
    </AbsoluteFill>
  );
};
