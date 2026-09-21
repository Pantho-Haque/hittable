// Package terminal is an integrated shell panel: the user's $SHELL runs on a
// pseudo-terminal, its output is parsed by a VT100 emulator (vt10x), and the
// resulting cell grid is rendered into the TUI. Keystrokes are translated to
// the byte sequences a real terminal would send.
package terminal

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

// OutputMsg is sent to the program whenever the shell produced output.
type OutputMsg struct{}

// vt10x attribute bits (unexported there; mirrored here).
const (
	attrReverse   = 1 << 0
	attrUnderline = 1 << 1
	attrBold      = 1 << 2
	attrItalic    = 1 << 4
)

type Terminal struct {
	Open    bool
	Focused bool
	Cols    int
	Rows    int

	dir    string
	shell  string
	notify func(tea.Msg)

	mu      sync.Mutex
	vt      vt10x.Terminal
	ptmx    *os.File
	cmd     *exec.Cmd
	running bool
	exitMsg string
	pending atomic.Bool
}

// New prepares a terminal rooted at dir. The shell starts on first Start.
func New(dir string, notify func(tea.Msg)) *Terminal {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return &Terminal{dir: dir, shell: shell, notify: notify, Cols: 80, Rows: 10}
}

// Start launches the shell if it is not running.
func (t *Terminal) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		return nil
	}
	cmd := exec.Command(t.shell, "-l")
	cmd.Dir = t.dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "HITTABLE=1")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(t.Rows), Cols: uint16(t.Cols)})
	if err != nil {
		return err
	}
	t.vt = vt10x.New(vt10x.WithSize(t.Cols, t.Rows))
	t.ptmx = ptmx
	t.cmd = cmd
	t.running = true
	t.exitMsg = ""

	vt, notify := t.vt, t.notify
	go func() {
		br := bufio.NewReader(ptmx)
		for {
			if err := vt.Parse(br); err != nil {
				break
			}
			t.wake()
		}
		_ = cmd.Wait()
		t.mu.Lock()
		t.running = false
		t.exitMsg = "[process exited — press any key to restart]"
		t.mu.Unlock()
		if notify != nil {
			notify(OutputMsg{})
		}
	}()
	return nil
}

// wake asks the program to re-render, coalescing bursts of output.
func (t *Terminal) wake() {
	if t.notify == nil || !t.pending.CompareAndSwap(false, true) {
		return
	}
	go func() {
		time.Sleep(8 * time.Millisecond)
		t.pending.Store(false)
		t.notify(OutputMsg{})
	}()
}

// Close kills the shell.
func (t *Terminal) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	if t.ptmx != nil {
		_ = t.ptmx.Close()
	}
	t.running = false
}

func (t *Terminal) SetSize(cols, rows int) {
	if cols < 10 {
		cols = 10
	}
	if rows < 2 {
		rows = 2
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if cols == t.Cols && rows == t.Rows {
		return
	}
	t.Cols, t.Rows = cols, rows
	if t.running {
		t.vt.Resize(cols, rows)
		_ = pty.Setsize(t.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	}
}

// Write sends raw bytes to the shell.
func (t *Terminal) Write(b []byte) {
	t.mu.Lock()
	p := t.ptmx
	running := t.running
	t.mu.Unlock()
	if running && p != nil {
		_, _ = p.Write(b)
	}
}

// SendKey translates a key event into terminal input bytes.
func (t *Terminal) SendKey(msg tea.KeyMsg) {
	t.mu.Lock()
	running := t.running
	t.mu.Unlock()
	if !running {
		_ = t.Start()
		return
	}
	if msg.String() == "ctrl+v" {
		if s, err := clipboard.ReadAll(); err == nil {
			t.Write([]byte(s))
		}
		return
	}
	if b := keyBytes(msg); len(b) > 0 {
		t.Write(b)
	}
}

func keyBytes(msg tea.KeyMsg) []byte {
	var s string
	switch msg.Type {
	case tea.KeyRunes:
		s = string(msg.Runes)
	case tea.KeySpace:
		s = " "
	case tea.KeyEnter:
		s = "\r"
	case tea.KeyBackspace:
		s = "\x7f"
	case tea.KeyTab:
		s = "\t"
	case tea.KeyShiftTab:
		s = "\x1b[Z"
	case tea.KeyEsc:
		s = "\x1b"
	case tea.KeyUp:
		s = "\x1b[A"
	case tea.KeyDown:
		s = "\x1b[B"
	case tea.KeyRight:
		s = "\x1b[C"
	case tea.KeyLeft:
		s = "\x1b[D"
	case tea.KeyHome:
		s = "\x1b[H"
	case tea.KeyEnd:
		s = "\x1b[F"
	case tea.KeyPgUp:
		s = "\x1b[5~"
	case tea.KeyPgDown:
		s = "\x1b[6~"
	case tea.KeyDelete:
		s = "\x1b[3~"
	case tea.KeyInsert:
		s = "\x1b[2~"
	case tea.KeyF1, tea.KeyF2, tea.KeyF3, tea.KeyF4:
		s = "\x1bO" + string(rune('P'+int(msg.Type-tea.KeyF1)))
	default:
		name := msg.String()
		if strings.HasPrefix(name, "ctrl+") && len(name) == 6 {
			c := name[5]
			switch {
			case c >= 'a' && c <= 'z':
				s = string(rune(c - 'a' + 1))
			case c == '@':
				s = "\x00"
			case c == '[':
				s = "\x1b"
			case c == '\\':
				s = "\x1c"
			case c == ']':
				s = "\x1d"
			case c == '^':
				s = "\x1e"
			case c == '_':
				s = "\x1f"
			}
		}
	}
	if s == "" {
		return nil
	}
	if msg.Alt {
		s = "\x1b" + s
	}
	return []byte(s)
}

// ---------- rendering ----------

func sgr(fg, bg vt10x.Color, mode int16) string {
	var parts []string
	if mode&attrBold != 0 {
		parts = append(parts, "1")
	}
	if mode&attrItalic != 0 {
		parts = append(parts, "3")
	}
	if mode&attrUnderline != 0 {
		parts = append(parts, "4")
	}
	if mode&attrReverse != 0 {
		parts = append(parts, "7")
	}
	parts = append(parts, colorSGR(fg, 30, 38, 39))
	parts = append(parts, colorSGR(bg, 40, 48, 49))
	return "\x1b[0;" + strings.Join(parts, ";") + "m"
}

func colorSGR(c vt10x.Color, base, ext, def int) string {
	switch {
	case c < 8:
		return fmt.Sprint(base + int(c))
	case c < 16:
		return fmt.Sprint(base + 60 + int(c) - 8)
	case c < 256:
		return fmt.Sprintf("%d;5;%d", ext, c)
	case c < 1<<24:
		return fmt.Sprintf("%d;2;%d;%d;%d", ext, (c>>16)&0xff, (c>>8)&0xff, c&0xff)
	}
	return fmt.Sprint(def)
}

// View renders the grid as Rows lines of exactly Cols cells.
func (t *Terminal) View() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.running || t.vt == nil {
		msg := t.exitMsg
		if msg == "" {
			msg = "starting shell…"
		}
		lines := []string{msg}
		for len(lines) < t.Rows {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}
	t.vt.Lock()
	defer t.vt.Unlock()
	cur := t.vt.Cursor()
	showCur := t.Focused && t.vt.CursorVisible()
	cols, rows := t.vt.Size()
	var out []string
	for y := 0; y < rows && y < t.Rows; y++ {
		var sb strings.Builder
		var last string
		for x := 0; x < cols && x < t.Cols; x++ {
			g := t.vt.Cell(x, y)
			mode := g.Mode
			if showCur && x == cur.X && y == cur.Y {
				mode ^= attrReverse
			}
			if code := sgr(g.FG, g.BG, mode); code != last {
				sb.WriteString(code)
				last = code
			}
			ch := g.Char
			if ch == 0 {
				ch = ' '
			}
			sb.WriteRune(ch)
		}
		sb.WriteString("\x1b[0m")
		out = append(out, sb.String())
	}
	for len(out) < t.Rows {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

// Snapshot returns the plain-text grid (for tests).
func (t *Terminal) Snapshot() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.running || t.vt == nil {
		return ""
	}
	return t.vt.String() // locks internally
}
