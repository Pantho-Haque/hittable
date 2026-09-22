import React from "react";
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from "remotion";
import { BG, CYAN, Mark, Wordmark } from "./Brand";
import { MONO } from "./Terminal";

export const Intro: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps, durationInFrames } = useVideoConfig();

  const rise = spring({ frame, fps, config: { damping: 200 } });
  const markScale = interpolate(rise, [0, 1], [0.7, 1]);
  const wordIn = spring({ frame: frame - 10, fps, config: { damping: 200 } });
  const lineIn = spring({ frame: frame - 20, fps, config: { damping: 200 } });
  // Hand off to the walkthrough rather than cutting.
  const out = interpolate(frame, [durationInFrames - 12, durationInFrames], [1, 0], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });

  return (
    <AbsoluteFill style={{ background: BG, alignItems: "center", justifyContent: "center", opacity: out }}>
      <div style={{ transform: `scale(${markScale})`, opacity: rise }}>
        <Mark size={190} />
      </div>
      <div style={{ height: 46 }} />
      <div style={{ opacity: wordIn, transform: `translateY(${(1 - wordIn) * 18}px)` }}>
        <Wordmark size={62} />
      </div>
      <div
        style={{
          marginTop: 26,
          fontFamily: MONO,
          fontSize: 27,
          color: "#8b8fa3",
          opacity: lineIn,
          transform: `translateY(${(1 - lineIn) * 14}px)`,
        }}
      >
        a terminal API client · <span style={{ color: CYAN }}>.hit</span> files, shared with the web app
      </div>
    </AbsoluteFill>
  );
};
