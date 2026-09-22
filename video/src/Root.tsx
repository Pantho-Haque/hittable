import React from "react";
import { AbsoluteFill, Composition, Series, staticFile } from "remotion";
import { Audio } from "remotion";
import { Intro } from "./Intro";
import { Outro } from "./Outro";
import { Walkthrough } from "./Walkthrough";
import { Teaser, timeline } from "./Teaser";
import { durationMs } from "./capture";

const FPS = 30;
const INTRO = 5 * FPS;
const OUTRO = 6 * FPS;
const BODY = Math.ceil((durationMs / 1000) * FPS);
const TOTAL = INTRO + BODY + OUTRO;

/** The same beat as the teaser, held back under the narration captions. */
const Track: React.FC<{ frames: number }> = ({ frames }) => (
  <Audio
    src={staticFile("beat-long.wav")}
    volume={(f) =>
      Math.min(f / (1.5 * FPS), 1) * Math.min((frames - f) / (3 * FPS), 1) * 0.42
    }
  />
);

const Walkthrough2Min: React.FC = () => (
  <AbsoluteFill>
    <Track frames={TOTAL} />
    <Series>
      <Series.Sequence durationInFrames={INTRO}>
        <Intro />
      </Series.Sequence>
      <Series.Sequence durationInFrames={BODY}>
        <Walkthrough />
      </Series.Sequence>
      <Series.Sequence durationInFrames={OUTRO}>
        <Outro />
      </Series.Sequence>
    </Series>
  </AbsoluteFill>
);

export const RemotionRoot: React.FC = () => (
  <>
    <Composition
      id="Walkthrough"
      component={Walkthrough2Min}
      durationInFrames={TOTAL}
      fps={FPS}
      width={1920}
      height={1080}
    />
    <Composition
      id="Teaser"
      component={Teaser}
      durationInFrames={timeline.total}
      fps={FPS}
      width={1920}
      height={1080}
    />
  </>
);
