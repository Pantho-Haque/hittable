// Command capture drives the real hittable TUI and records every frame it
// draws, so the promo video is rendered from the app's own output instead of a
// screen recording: no capture artefacts, reproducible, and crisp at any zoom
// because the video renders text rather than pixels.
package main

import (
    "bufio"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/hinshun/vt10x"
    "github.com/muesli/termenv"

    "github.com/hittable/shellapp/ui"
    "github.com/hittable/shellapp/ui/components/explorer"
    "github.com/hittable/shellapp/ui/screens"
)

type session struct {
    in   io.Writer
    vt   vt10x.Terminal
    p    *tea.Program
    cols int
    rows int

    rec   *recorder
    start time.Time
}

func newSession(root string, cols, rows int) *session {
    // The app is not attached to a TTY here, so lipgloss would otherwise
    // decide the terminal has no colour and render everything plain.
    lipgloss.SetColorProfile(termenv.TrueColor)
    lipgloss.SetHasDarkBackground(true)
    // Emoji rather than Nerd Font glyphs: the video has no Nerd Font to fall
    // back on, and private-use code points would render as tofu.
    explorer.IconMode = "emoji"

    inR, inW := io.Pipe()
    outR, outW := io.Pipe()
    app := ui.NewApp(root)
    p := tea.NewProgram(app,
        tea.WithInput(&ui.MouseSafeReader{R: inR}),
        tea.WithOutput(outW),
        tea.WithMouseAllMotion(),
        tea.WithoutSignals())
    screens.SetTeaProgram(p)

    vt := vt10x.New(vt10x.WithSize(cols, rows))
    go func() {
        br := bufio.NewReader(outR)
        for {
            if err := vt.Parse(br); err != nil {
                return
            }
        }
    }()
    go func() { _, _ = p.Run(); outW.Close() }()

    s := &session{in: inW, vt: vt, p: p, cols: cols, rows: rows, start: time.Now()}
    time.Sleep(200 * time.Millisecond)
    p.Send(tea.WindowSizeMsg{Width: cols, Height: rows})
    time.Sleep(300 * time.Millisecond)
    return s
}

func (s *session) ms() int { return int(time.Since(s.start).Milliseconds()) }

func (s *session) wait(d time.Duration) { time.Sleep(d) }

// key sends a key by name, or the character itself for a plain rune.
func (s *session) key(name string) {
    seq, ok := keySeq[name]
    if !ok {
        if len([]rune(name)) != 1 {
            panic("capture: unknown key " + name)
        }
        seq = name
    }
    fmt.Fprint(s.in, seq)
    s.wait(160 * time.Millisecond)
}

func (s *session) keys(names ...string) {
    for _, n := range names {
        s.key(n)
    }
}

// typeText sends text one rune at a time so the video shows it being typed.
func (s *session) typeText(text string, perRune time.Duration) {
    for _, r := range text {
        fmt.Fprint(s.in, string(r))
        s.wait(perRune)
    }
}

func (s *session) click(x, y int) {
    fmt.Fprintf(s.in, "\x1b[<0;%d;%dM\x1b[<0;%d;%dm", x+1, y+1, x+1, y+1)
    s.wait(220 * time.Millisecond)
}

// hover moves the mouse without pressing, which is what drives the hover
// styling the video wants to show off.
func (s *session) hover(x, y int) {
    fmt.Fprintf(s.in, "\x1b[<35;%d;%dM", x+1, y+1)
    s.wait(180 * time.Millisecond)
}

func (s *session) screen() []string {
    s.vt.Lock()
    defer s.vt.Unlock()
    out := make([]string, s.rows)
    for y := 0; y < s.rows; y++ {
        var sb strings.Builder
        for x := 0; x < s.cols; x++ {
            c := s.vt.Cell(x, y).Char
            if c == 0 {
                c = ' '
            }
            sb.WriteRune(c)
        }
        out[y] = sb.String()
    }
    return out
}

// find locates on-screen text, returning its display column and row.
func (s *session) find(text string) (int, int, bool) {
    for y, line := range s.screen() {
        if x := strings.Index(line, text); x >= 0 {
            return lipgloss.Width(line[:x]), y, true
        }
    }
    return 0, 0, false
}

// clickOn clicks the first cell of on-screen text, offset by dx/dy.
func (s *session) clickOn(text string, dx, dy int) bool {
    x, y, ok := s.find(text)
    if !ok {
        fmt.Fprintf(os.Stderr, "capture: %q not on screen\n", text)
        return false
    }
    s.click(x+dx, y+dy)
    return true
}

func (s *session) hoverOn(text string, dx, dy int) bool {
    x, y, ok := s.find(text)
    if !ok {
        return false
    }
    s.hover(x+dx, y+dy)
    return true
}

func (s *session) close() {
    s.p.Kill()
    time.Sleep(100 * time.Millisecond)
}

var keySeq = map[string]string{
    "enter": "\r", "esc": "\x1b", "tab": "\t", "space": " ",
    "up": "\x1b[A", "down": "\x1b[B", "right": "\x1b[C", "left": "\x1b[D",
    "pgup": "\x1b[5~", "pgdown": "\x1b[6~",
    "ctrl+b": "\x02", "ctrl+c": "\x03", "ctrl+d": "\x04", "ctrl+e": "\x05",
    "ctrl+f": "\x06", "ctrl+g": "\x07", "ctrl+j": "\x0a", "ctrl+l": "\x0c",
    "ctrl+n": "\x0e", "ctrl+p": "\x10", "ctrl+r": "\x12", "ctrl+s": "\x13",
    "ctrl+t": "\x14", "ctrl+y": "\x19", "ctrl+z": "\x1a",
    "alt+f": "\x1bf", "alt+g": "\x1bg", "alt+b": "\x1bb", "alt+z": "\x1bz",
    "F5": "\x1b[15~", "F1": "\x1bOP",
}
