# hittable — walkthrough video

A ~2 minute tour of the shell app, rendered with [Remotion](https://remotion.dev).

The terminal you see is **not a screen recording**. `shellapp/tools/capture`
runs the real TUI against a vt10x emulator, drives a scripted session, and
records every frame it draws as coloured text. Remotion replays that as a live
DOM grid, so the footage is reproducible, has no capture artefacts, and stays
sharp at any zoom — it is text being scaled, not pixels.

## Render it

```bash
pnpm install          # once
pnpm capture          # drive the app, write public/capture.json
pnpm music            # synthesise the backing track
pnpm build            # -> out/hittable.mp4       (~2 min tour)
pnpm build:30s        # -> out/hittable-30s.mp4   (30s teaser)
pnpm studio           # preview and scrub instead
```

## How the pieces fit

| Piece | What it does |
| ----- | ------------ |
| `shellapp/tools/capture/script.go` | the walkthrough: one `scene` per beat, with its caption and the cell rectangle the camera zooms to |
| `shellapp/tools/capture/fixture.go` | the demo project — a collection, notes, and a git repo with history, branches and uncommitted work |
| `shellapp/tools/capture/server.go` | a local API, so the request scene always gets a real response and never depends on the network |
| `shellapp/tools/capture/recorder.go` | samples the emulator and stores only the rows that changed |
| `src/capture.ts` | replays those deltas into a full screen for any timestamp |
| `src/Terminal.tsx` | draws one frame as styled spans |
| `src/Walkthrough.tsx` | turns each scene's focus rectangle into a camera move, and draws the captions |
| `src/Teaser.tsx` | the 30s cut: seven shots, each one bar long, so every cut lands on the beat |
| `scripts/music.py` | the backing track — a four-on-the-floor loop over I-V-vi-IV at 124 BPM |

## Editing the tour

Change `script.go` and re-run `pnpm capture`. A scene is:

```go
{
    Name: "Split diff", Caption: "Side by side, wrapped, resizable",
    Note:  "v splits · z wraps long lines · drag the divider",
    Focus: [4]int{0, 13, 0, 21}, // x, y, w, h in terminal cells; 0 = to the edge
    act: func(s *session) { s.key("v"); s.wait(think) },
},
```

A focus rectangle that spans a whole axis cannot zoom without cropping, so the
camera stays wide for it — bound **both** width and height to get a zoom.

## Notes

- Icons are captured in emoji mode: the Nerd Font glyphs the app prefers would
  render as tofu without that font installed in the browser.
- The music is synthesised by `scripts/music.py`: drums built from shaped noise
  and a pitch-swept sine, a bass line and a sixteenth-note arpeggio. No samples,
  no licence. The teaser's edit is derived from the same 124 BPM grid
  (`BAR` in `Teaser.tsx`), so changing the tempo moves the cuts with it.
- Remotion's bundled Chrome download fails to extract on some machines;
  `remotion.config.ts` falls back to an installed headless shell or Chrome, and
  `REMOTION_BROWSER=/path/to/chrome` overrides it.
