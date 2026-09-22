import React from "react";
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from "remotion";
import { BG, CYAN, Mark, Wordmark } from "./Brand";
import { MONO } from "./Terminal";

const lines: [string, string][] = [
  ["install", "curl -fsSL .../Pantho-Haque/hittable/main/install.sh | sh"],
  ["hittable", "open the current directory"],
  ["hittable init", "scaffold a collection"],
];

export const Outro: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const inn = spring({ frame, fps, config: { damping: 200 } });

  return (
    <AbsoluteFill style={{ background: BG, alignItems: "center", justifyContent: "center" }}>
      <div style={{ opacity: inn, transform: `scale(${interpolate(inn, [0, 1], [0.85, 1])})` }}>
        <Mark size={140} />
      </div>
      <div style={{ height: 34 }} />
      <Wordmark size={52} />
      <div style={{ marginTop: 40, fontFamily: MONO, fontSize: 24, lineHeight: "40px" }}>
        {lines.map(([cmd, note], i) => {
          const s = spring({ frame: frame - 12 - i * 7, fps, config: { damping: 200 } });
          return (
            <div key={cmd} style={{ opacity: s, transform: `translateY(${(1 - s) * 12}px)` }}>
              <span style={{ color: CYAN }}>{cmd.padEnd(15)}</span>
              <span style={{ color: "#8b8fa3" }}>{note}</span>
            </div>
          );
        })}
      </div>
      <div style={{ marginTop: 44, fontFamily: MONO, fontSize: 21, color: "#6272a4" }}>
        github.com/Pantho-Haque/hittable · one binary · your files stay plain JSON
      </div>
    </AbsoluteFill>
  );
};
