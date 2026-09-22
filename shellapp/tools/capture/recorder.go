package main

import (
    "fmt"
    "sync"
    "time"

    "github.com/hinshun/vt10x"
)

// A run is a stretch of one row sharing the same colours: [text, fg, bg, attr].
// Encoded as an array rather than an object to keep the JSON small — a
// three-minute capture is tens of thousands of runs.
type run [4]any

// frame carries only the rows that changed since the previous one. The player
// accumulates them, so the file stays small without losing any redraw.
type frame struct {
    Ms   int              `json:"ms"`
    Rows map[string][]run `json:"rows"`
}

type recorder struct {
    mu     sync.Mutex
    frames []frame
    prev   []string // last serialised form of each row, for change detection
    cols   int
    rows   int
}

func newRecorder(cols, rows int) *recorder {
    return &recorder{prev: make([]string, rows), cols: cols, rows: rows}
}

// vt10x attribute bits (unexported there; mirrored here as the app does).
const (
    attrReverse   = 1 << 0
    attrUnderline = 1 << 1
    attrBold      = 1 << 2
    attrItalic    = 1 << 4
)

// capture samples the emulator and appends a frame for whatever changed.
func (r *recorder) capture(t vt10x.Terminal, ms int) {
    t.Lock()
    rows := make([][]run, r.rows)
    keys := make([]string, r.rows)
    for y := 0; y < r.rows; y++ {
        rows[y], keys[y] = r.row(t, y)
    }
    t.Unlock()

    r.mu.Lock()
    defer r.mu.Unlock()
    changed := map[string][]run{}
    for y := 0; y < r.rows; y++ {
        if keys[y] != r.prev[y] {
            changed[fmt.Sprint(y)] = rows[y]
            r.prev[y] = keys[y]
        }
    }
    if len(changed) == 0 {
        return
    }
    r.frames = append(r.frames, frame{Ms: ms, Rows: changed})
}

// row collapses a line into style runs, plus a key used to spot changes.
func (r *recorder) row(t vt10x.Terminal, y int) ([]run, string) {
    var out []run
    var key string
    var text string
    var fg, bg any
    var attr int
    flush := func() {
        if text == "" {
            return
        }
        out = append(out, run{text, fg, bg, attr})
        key += fmt.Sprintf("%q|%v|%v|%d;", text, fg, bg, attr)
        text = ""
    }
    for x := 0; x < r.cols; x++ {
        g := t.Cell(x, y)
        ch := g.Char
        if ch == 0 {
            ch = ' '
        }
        cfg, cbg := colorOf(g.FG), colorOf(g.BG)
        ca := int(g.Mode) & (attrReverse | attrUnderline | attrBold | attrItalic)
        if ca&attrReverse != 0 { // draw reverse video as swapped colours
            cfg, cbg = cbg, cfg
            ca &^= attrReverse
        }
        if text != "" && (cfg != fg || cbg != bg || ca != attr) {
            flush()
        }
        fg, bg, attr = cfg, cbg, ca
        text += string(ch)
    }
    flush()
    return out, key
}

// colorOf turns a vt10x colour into something the player can use: "#rrggbb"
// for a true colour, the palette index for the 256-colour range, or nil for
// the terminal default.
func colorOf(c vt10x.Color) any {
    switch {
    case c < 256:
        return int(c)
    case c < 1<<24:
        return fmt.Sprintf("#%02x%02x%02x", (c>>16)&0xff, (c>>8)&0xff, c&0xff)
    }
    return nil
}

// run samples on a fixed interval until stopped. Frames are only appended when
// something actually changed, so an idle beat costs nothing.
func (r *recorder) run(s *session, every time.Duration) (stop func()) {
    done := make(chan struct{})
    go func() {
        tick := time.NewTicker(every)
        defer tick.Stop()
        for {
            select {
            case <-done:
                return
            case <-tick.C:
                r.capture(s.vt, s.ms())
            }
        }
    }()
    return func() { close(done) }
}
