import React from "react";
import {
  AbsoluteFill, Audio, interpolate, Sequence, spring, staticFile,
  useCurrentFrame, useVideoConfig,
} from "remotion";
import { COLS, ROWS } from "./capture";
import { CELL_H, CELL_W, MONO, TERM_H, TERM_W, Terminal } from "./Terminal";
import { BG, CYAN, Mark, Wordmark } from "./Brand";

export const FPS = 30;
const BPM = 124;
/** Every cut lands on a bar line, so the edit moves with the track. */
export const BAR = (FPS * 60 * 4) / BPM; // 58.06 frames
const VW = 1920;
const VH = 1080;

type Clip = {
  /** Where to start in the capture, in ms. */
  at: number;
  bars: number;
  caption: string;
  note: string;
  focus: [number, number, number, number];
};

// The seven beats that read fastest — each one lands on a bar.
const CLIPS: Clip[] = [
  { at: 9700, bars: 1.5, caption: "Your API collection", note: "plain files, one tree",
    focus: [0, 0, 44, 22] },
  { at: 16200, bars: 1.5, caption: "Send it", note: "ctrl+r", focus: [34, 1, 98, 12] },
  { at: 18400, bars: 1.5, caption: "200 OK", note: "status · timing · size",
    focus: [34, 13, 98, 20] },
  { at: 41200, bars: 1.5, caption: "Markdown, live", note: "mermaid becomes a diagram",
    focus: [0, 0, 0, 0] },
  { at: 62200, bars: 1.5, caption: "Split diffs", note: "wrap · resize · stage",
    focus: [0, 12, 0, 22] },
  { at: 67600, bars: 1.5, caption: "Commit without leaving", note: "git, built in",
    focus: [0, 1, 0, 16] },
  { at: 91500, bars: 1.5, caption: "A real shell", note: "ctrl+j", focus: [0, 17, 0, 18] },
];

const INTRO_BARS = 2;
const OUTRO_BARS = 3; // 2 + 7*1.5 + 3 = 15.5 bars = exactly 30.0s

function frames(bars: number) {
  return Math.round(bars * BAR);
}

/** Clip boundaries, accumulated so rounding never drifts off the beat. */
export const timeline = (() => {
  let f = 0;
  const intro = { from: 0, len: frames(INTRO_BARS) };
  f += intro.len;
  const clips = CLIPS.map((c) => {
    const len = frames(c.bars);
    const seq = { clip: c, from: f, len };
    f += len;
    return seq;
  });
  const outro = { from: f, len: frames(OUTRO_BARS) };
  return { intro, clips, outro, total: f + outro.len };
})();

function viewFor(focus: [number, number, number, number]) {
  const band = 190;
  const usableH = VH - band;
  const whole = Math.min(VW / TERM_W, usableH / TERM_H) * 0.97;
  const lift = -band / 2;
  const [fx, fy] = focus;
  // A zero width or height means "out to the edge".
  const fw = focus[2] > 0 ? focus[2] : COLS - fx;
  const fh = focus[3] > 0 ? focus[3] : ROWS - fy;
  const scale = Math.min(
    Math.max(Math.min((VW * 0.92) / (fw * CELL_W), (usableH * 0.95) / (fh * CELL_H)), whole),
    2.9,
  );
  if (scale <= whole * 1.03) return { scale: whole, x: 0, y: lift };
  const cx = (fx + fw / 2) * CELL_W - TERM_W / 2;
  const cy = (fy + fh / 2) * CELL_H - TERM_H / 2;
  return { scale, x: -cx * scale, y: -cy * scale + lift };
}

const Shot: React.FC<{ clip: Clip }> = ({ clip }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const view = viewFor(clip.focus);

  // A short push-in on the cut: the picture lands with the kick.
  const punch = spring({ frame, fps, config: { damping: 14, mass: 0.5, stiffness: 120 } });
  const scale = view.scale * interpolate(punch, [0, 1], [1.05, 1]);
  const pop = spring({ frame: frame - 2, fps, config: { damping: 16, mass: 0.6 } });

  return (
    <AbsoluteFill style={{ background: BG }}>
      <AbsoluteFill style={{ alignItems: "center", justifyContent: "center" }}>
        <div
          style={{
            transform: `translate(${view.x}px, ${view.y}px) scale(${scale})`,
            boxShadow: "0 40px 120px rgba(0,0,0,0.6)",
            borderRadius: 10,
            opacity: interpolate(punch, [0, 0.4], [0, 1], { extrapolateRight: "clamp" }),
          }}
        >
          <Terminal ms={clip.at + (frame / fps) * 1000} />
        </div>
      </AbsoluteFill>

      <AbsoluteFill style={{ justifyContent: "flex-end", pointerEvents: "none" }}>
        <div
          style={{
            padding: "150px 90px 58px",
            background:
              "linear-gradient(to top, rgba(14,15,20,0.95) 26%, rgba(14,15,20,0) 100%)",
          }}
        >
          <div
            style={{
              fontFamily: MONO, fontSize: 62, fontWeight: 700, color: "#ffffff",
              letterSpacing: -1,
              transform: `translateY(${(1 - pop) * 26}px)`,
              opacity: pop,
            }}
          >
            {clip.caption}
          </div>
          <div
            style={{
              fontFamily: MONO, fontSize: 28, color: CYAN, marginTop: 12,
              opacity: pop * 0.9,
              transform: `translateY(${(1 - pop) * 14}px)`,
            }}
          >
            {clip.note}
          </div>
        </div>
      </AbsoluteFill>
    </AbsoluteFill>
  );
};

const Hook: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps, durationInFrames } = useVideoConfig();
  // The track lifts for one bar and drops on the second: land the mark there.
  const drop = Math.round(BAR);
  const rise = spring({ frame, fps, config: { damping: 200 } });
  const hit = spring({ frame: frame - drop, fps, config: { damping: 11, mass: 0.6, stiffness: 140 } });
  const out = interpolate(frame, [durationInFrames - 6, durationInFrames], [1, 0], {
    extrapolateLeft: "clamp", extrapolateRight: "clamp",
  });

  return (
    <AbsoluteFill
      style={{ background: BG, alignItems: "center", justifyContent: "center", opacity: out }}
    >
      <div style={{ transform: `scale(${interpolate(rise, [0, 1], [0.6, 1]) * interpolate(hit, [0, 1], [1, 1.12])})` }}>
        <Mark size={200} />
      </div>
      <div style={{ height: 44, opacity: hit }} />
      <div style={{ opacity: hit, transform: `translateY(${(1 - hit) * 24}px)` }}>
        <Wordmark size={66} />
      </div>
      <div
        style={{
          marginTop: 22, fontFamily: MONO, fontSize: 29, color: "#9aa0b4",
          opacity: interpolate(frame, [drop + 6, drop + 16], [0, 1], {
            extrapolateLeft: "clamp", extrapolateRight: "clamp",
          }),
        }}
      >
        an API client that lives in your terminal
      </div>
    </AbsoluteFill>
  );
};

const CallToAction: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const inn = spring({ frame, fps, config: { damping: 200 } });
  const cmd = spring({ frame: frame - 8, fps, config: { damping: 18 } });

  return (
    <AbsoluteFill style={{ background: BG, alignItems: "center", justifyContent: "center" }}>
      <div style={{ opacity: inn, transform: `scale(${interpolate(inn, [0, 1], [0.8, 1])})` }}>
        <Mark size={130} />
      </div>
      <div style={{ height: 30 }} />
      <Wordmark size={50} />
      <div
        style={{
          marginTop: 46,
          padding: "22px 40px",
          borderRadius: 14,
          background: "#15161d",
          border: "1px solid #2b2d3a",
          fontFamily: MONO,
          fontSize: 24,
          color: "#e7e9f0",
          opacity: cmd,
          transform: `translateY(${(1 - cmd) * 16}px)`,
        }}
      >
        <span style={{ color: CYAN }}>curl -fsSL </span>
        raw.githubusercontent.com/Pantho-Haque/hittable/main/install.sh
        <span style={{ color: CYAN }}> | sh</span>
      </div>
      <div style={{ marginTop: 26, fontFamily: MONO, fontSize: 23, color: "#6d7488", opacity: cmd }}>
        github.com/Pantho-Haque/hittable
      </div>
    </AbsoluteFill>
  );
};

export const Teaser: React.FC = () => (
  <AbsoluteFill style={{ background: BG }}>
    <Audio src={staticFile("beat.wav")} volume={0.85} />
    <Sequence from={timeline.intro.from} durationInFrames={timeline.intro.len}>
      <Hook />
    </Sequence>
    {timeline.clips.map((s) => (
      <Sequence key={s.from} from={s.from} durationInFrames={s.len}>
        <Shot clip={s.clip} />
      </Sequence>
    ))}
    <Sequence from={timeline.outro.from} durationInFrames={timeline.outro.len}>
      <CallToAction />
    </Sequence>
  </AbsoluteFill>
);
