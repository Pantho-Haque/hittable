package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "os"
    "path/filepath"
    "time"
)

// scene is one beat of the walkthrough. Focus is the cell rectangle the video
// zooms into; an empty rectangle means the whole screen.
type scene struct {
    Name    string `json:"name"`
    Caption string `json:"caption"`
    Note    string `json:"note,omitempty"`
    Focus   [4]int `json:"focus"` // x, y, w, h in cells
    StartMs int    `json:"startMs"`
    EndMs   int    `json:"endMs"`

    act func(s *session)
}

type capture struct {
    Cols   int     `json:"cols"`
    Rows   int     `json:"rows"`
    Scenes []scene `json:"scenes"`
    Frames []frame `json:"frames"`
}

func main() {
    out := flag.String("out", "../video/public/capture.json", "where to write the capture")
    cols := flag.Int("cols", 132, "terminal columns")
    rows := flag.Int("rows", 36, "terminal rows")
    flag.Parse()

    tmp, err := os.MkdirTemp("", "hittable-capture-")
    if err != nil {
        fail(err)
    }
    defer os.RemoveAll(tmp)
    // A stable, presentable name: it shows in the title bar and breadcrumb.
    root := filepath.Join(tmp, "acme-api")
    if err := os.MkdirAll(root, 0o755); err != nil {
        fail(err)
    }
    api := startAPI()
    defer api.Close()
    if err := buildFixture(root, api.URL); err != nil {
        fail(err)
    }

    s := newSession(root, *cols, *rows)
    defer s.close()
    rec := newRecorder(*cols, *rows)
    stop := rec.run(s, 40*time.Millisecond)

    scenes := script()
    for i := range scenes {
        sc := &scenes[i]
        sc.StartMs = s.ms()
        fmt.Fprintf(os.Stderr, "  %-22s %6dms\n", sc.Name, sc.StartMs)
        sc.act(s)
        s.wait(700 * time.Millisecond) // let the last beat breathe
        sc.EndMs = s.ms()
    }
    stop()
    rec.capture(s.vt, s.ms())

    c := capture{Cols: *cols, Rows: *rows, Scenes: scenes, Frames: rec.frames}
    if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
        fail(err)
    }
    f, err := os.Create(*out)
    if err != nil {
        fail(err)
    }
    defer f.Close()
    if err := json.NewEncoder(f).Encode(c); err != nil {
        fail(err)
    }
    st, _ := f.Stat()
    fmt.Fprintf(os.Stderr, "\n%d scenes, %d frames, %.1fs, %.1f MB -> %s\n",
        len(scenes), len(rec.frames), float64(scenes[len(scenes)-1].EndMs)/1000,
        float64(st.Size())/(1<<20), *out)
}

func fail(err error) {
    fmt.Fprintln(os.Stderr, "capture:", err)
    os.Exit(1)
}
