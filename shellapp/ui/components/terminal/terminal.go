// Package terminal is an integrated shell panel: the user's $SHELL runs on a
// pseudo-terminal, its output is parsed by a VT100 emulator (vt10x), and the
// resulting cell grid is rendered into the TUI. Keystrokes are translated to
// the byte sequences a real terminal would send.
package terminal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

	// Scrollback: lines that scrolled off the top (rendered), plus how far
	// the user has scrolled back into them.
	scrollback   []string
	ScrollOffset int
	prevPlain    []string
	prevStyled   []string
	prevCursorY  int

	// Mouse text selection, in view coordinates {row, col}. Anchored on
	// press and dropped whenever the view scrolls under it.
	selecting  bool
	selA, selB [2]int

	// Bracketed paste: tracked from the shell's DECSET 2004 requests.
	bracketed atomic.Bool
	writeCh   chan []byte
}

const maxScrollback = 5000

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
	t.scrollback, t.prevPlain, t.prevStyled = nil, nil, nil
	t.ScrollOffset = 0
	t.bracketed.Store(false)

	// Serialised writer so a large paste never blocks the UI goroutine.
	t.writeCh = make(chan []byte, 256)
	go func(w *os.File, ch chan []byte) {
		for b := range ch {
			for len(b) > 0 {
				n, err := w.Write(b)
				if err != nil {
					return
				}
				b = b[n:]
			}
		}
	}(ptmx, t.writeCh)

	vt, notify := t.vt, t.notify
	go func() {
		// Feed the emulator one output line at a time so scrollback capture
		// sees every row that scrolls off, even in large bursts.
		src := &modeSniffer{r: ptmx, t: t}
		buf := make([]byte, 32*1024)
		for {
			n, err := src.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				for len(chunk) > 0 {
					// Write the text before a newline, capture, then the
					// newline itself (which is what scrolls), capture again.
					i := bytes.IndexByte(chunk, '\n')
					if i < 0 {
						i = len(chunk)
					}
					if i > 0 {
						if _, werr := vt.Write(chunk[:i]); werr != nil {
							break
						}
						t.captureScrollback()
					}
					if i < len(chunk) {
						if _, werr := vt.Write(chunk[i : i+1]); werr != nil {
							break
						}
						t.captureScrollback()
						i++
					}
					chunk = chunk[i:]
				}
				t.wake()
			}
			if err != nil {
				break
			}
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
	if t.writeCh != nil {
		close(t.writeCh)
		t.writeCh = nil
	}
	t.running = false
}

// modeSniffer watches the output stream for bracketed-paste mode changes.
type modeSniffer struct {
	r io.Reader
	t *Terminal
}

func (m *modeSniffer) Read(p []byte) (int, error) {
	n, err := m.r.Read(p)
	if n > 0 {
		if bytes.Contains(p[:n], []byte("\x1b[?2004h")) {
			m.t.bracketed.Store(true)
		}
		if bytes.Contains(p[:n], []byte("\x1b[?2004l")) {
			m.t.bracketed.Store(false)
		}
	}
	return n, err
}

// captureScrollback compares the new screen with the previous one; if the
// top k rows scrolled off (new[:rows-k] == prev[k:]) they are appended to
// the scrollback. Only considered when output happened on the bottom row,
// which excludes clears and full-screen redraws; the alternate screen is
// never captured.
// ponytail: more than `rows` lines scrolling in one chunk loses the middle.
func (t *Terminal) captureScrollback() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.vt == nil {
		return
	}
	t.vt.Lock()
	cols, rows := t.vt.Size()
	alt := t.vt.Mode()&vt10x.ModeAltScreen != 0
	cur := t.vt.Cursor()
	plain := make([]string, rows)
	styled := make([]string, rows)
	for y := 0; y < rows; y++ {
		plain[y], styled[y] = t.renderRowLocked(y, cols, -1)
	}
	t.vt.Unlock()

	if !alt && rows >= 3 && len(t.prevPlain) == rows && (cur.Y == rows-1 || t.prevCursorY == rows-1) && !equalRows(plain, t.prevPlain) {
		// The previous cursor row may have been rewritten by the same piece
		// that scrolled (e.g. a wrapped line), so it is left out of the
		// comparison. k must leave at least one row to compare, otherwise
		// two empty slices "match" and the whole screen is captured.
		for k := 1; k < rows-1; k++ {
			if equalRows(plain[:rows-1-k], t.prevPlain[k:rows-1]) {
				t.scrollback = append(t.scrollback, t.prevStyled[:k]...)
				if len(t.scrollback) > maxScrollback {
					t.scrollback = t.scrollback[len(t.scrollback)-maxScrollback:]
				}
				break
			}
		}
	}
	t.prevPlain, t.prevStyled, t.prevCursorY = plain, styled, cur.Y
}

func equalRows(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Scroll moves the view into the scrollback (positive = older).
func (t *Terminal) Scroll(delta int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.selecting = false // view coords move under the selection
	t.ScrollOffset += delta
	if t.ScrollOffset > len(t.scrollback) {
		t.ScrollOffset = len(t.scrollback)
	}
	if t.ScrollOffset < 0 {
		t.ScrollOffset = 0
	}
}

// ---------- mouse selection ----------
//
// The app runs with mouse tracking on, so the host terminal's own selection
// never sees these cells; the panel selects and copies them itself.

// SelectStart anchors a selection at a view cell, dropping any previous one.
func (t *Terminal) SelectStart(col, row int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.selecting = true
	t.selA, t.selB = [2]int{row, col}, [2]int{row, col}
}

// SelectTo extends the live selection to a view cell.
func (t *Terminal) SelectTo(col, row int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.selecting {
		t.selB = [2]int{row, col}
	}
}

func (t *Terminal) ClearSelection() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.selecting = false
}

// HasSelection reports a selection covering at least one cell.
func (t *Terminal) HasSelection() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, _, ok := t.selOrderedLocked()
	return ok
}

// selOrderedLocked returns the selection ends in reading order. t.mu held.
func (t *Terminal) selOrderedLocked() (a, b [2]int, ok bool) {
	if !t.selecting || t.selA == t.selB {
		return a, b, false
	}
	a, b = t.selA, t.selB
	if b[0] < a[0] || (b[0] == a[0] && b[1] < a[1]) {
		a, b = b, a
	}
	return a, b, true
}

// SelectionText is the selected cells as plain text, one line per row.
func (t *Terminal) SelectionText() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	a, b, ok := t.selOrderedLocked()
	if !ok {
		return ""
	}
	lines := t.viewLinesLocked()
	var out []string
	for y := max(a[0], 0); y <= b[0] && y < len(lines); y++ {
		c0, c1 := 0, t.Cols
		if y == a[0] {
			c0 = a[1]
		}
		if y == b[0] {
			c1 = b[1]
		}
		out = append(out, strings.TrimRight(ansi.Strip(ansi.Cut(lines[y], c0, c1)), " "))
	}
	return strings.Join(out, "\n")
}

// CopySelection puts the selection on the system clipboard and reports how
// many lines were copied (0 = nothing selected, or no clipboard).
func (t *Terminal) CopySelection() int {
	s := t.SelectionText()
	if s == "" || clipboard.WriteAll(s) != nil {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// ScrollbackLen reports how many lines are available above the screen.
func (t *Terminal) ScrollbackLen() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.scrollback)
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

// Write queues raw bytes for the shell (never blocks the caller).
func (t *Terminal) Write(b []byte) {
	t.mu.Lock()
	ch := t.writeCh
	running := t.running
	t.mu.Unlock()
	if !running || ch == nil {
		return
	}
	cp := append([]byte(nil), b...)
	select {
	case ch <- cp:
	default:
		go func() { ch <- cp }()
	}
}

// Paste sends text the way a terminal does: newlines become CR, and if the
// shell enabled bracketed paste the block is wrapped so multi-line text is
// inserted as one unit instead of executed line by line.
func (t *Terminal) Paste(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\r")
	text = strings.ReplaceAll(text, "\n", "\r")
	if t.bracketed.Load() {
		text = "\x1b[200~" + text + "\x1b[201~"
	}
	t.Write([]byte(text))
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
	t.mu.Lock()
	t.ScrollOffset = 0 // any key snaps back to the live screen
	t.mu.Unlock()
	if msg.Paste {
		t.Paste(string(msg.Runes))
		return
	}
	if msg.String() == "ctrl+v" {
		if s, err := clipboard.ReadAll(); err == nil {
			t.Paste(s)
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

// renderRowLocked renders one screen row (vt must be locked). cursorX >= 0
// draws the cursor cell reversed. Returns the plain text and styled text.
func (t *Terminal) renderRowLocked(y, cols, cursorX int) (string, string) {
	var plain, sb strings.Builder
	var last string
	for x := 0; x < cols && x < t.Cols; x++ {
		g := t.vt.Cell(x, y)
		mode := g.Mode
		if x == cursorX {
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
		plain.WriteRune(ch)
	}
	sb.WriteString("\x1b[0m")
	return strings.TrimRight(plain.String(), " "), sb.String()
}

// View renders Rows lines of exactly Cols cells: the live screen, or a
// window into the scrollback when the user has scrolled up.
func (t *Terminal) View() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := t.viewLinesLocked()
	if a, b, ok := t.selOrderedLocked(); ok {
		for y := max(a[0], 0); y <= b[0] && y < len(lines); y++ {
			c0, c1 := 0, t.Cols
			if y == a[0] {
				c0 = a[1]
			}
			if y == b[0] {
				c1 = b[1]
			}
			lines[y] = highlight(lines[y], c0, c1, t.Cols)
		}
	}
	return strings.Join(lines, "\n")
}

// highlight reverse-videos cells [c0,c1) of an already-styled row.
func highlight(line string, c0, c1, cols int) string {
	if c1 <= c0 {
		return line
	}
	mid := ansi.Strip(ansi.Cut(line, c0, c1))
	if n := c1 - c0 - lipgloss.Width(mid); n > 0 {
		mid += strings.Repeat(" ", n) // blank cells still highlight
	}
	return ansi.Cut(line, 0, c0) + "\x1b[7m" + mid + "\x1b[27m" + ansi.Cut(line, c1, cols)
}

// viewLinesLocked builds the Rows visible lines. t.mu must be held.
func (t *Terminal) viewLinesLocked() []string {
	if !t.running || t.vt == nil {
		msg := t.exitMsg
		if msg == "" {
			msg = "starting shell…"
		}
		lines := []string{msg}
		for len(lines) < t.Rows {
			lines = append(lines, "")
		}
		for i, l := range lines {
			if n := t.Cols - lipgloss.Width(l); n > 0 {
				lines[i] = l + strings.Repeat(" ", n)
			}
		}
		return lines
	}
	t.vt.Lock()
	cur := t.vt.Cursor()
	showCur := t.Focused && t.vt.CursorVisible() && t.ScrollOffset == 0
	cols, rows := t.vt.Size()
	screen := make([]string, 0, rows)
	for y := 0; y < rows && y < t.Rows; y++ {
		cx := -1
		if showCur && y == cur.Y {
			cx = cur.X
		}
		_, styled := t.renderRowLocked(y, cols, cx)
		screen = append(screen, styled)
	}
	t.vt.Unlock()

	var out []string
	if t.ScrollOffset > 0 {
		// Window ends ScrollOffset lines above the bottom of the live screen.
		all := append(append([]string{}, t.scrollback...), screen...)
		end := len(all) - t.ScrollOffset
		start := end - t.Rows
		if start < 0 {
			start = 0
		}
		out = append(out, all[start:end]...)
	} else {
		out = screen
	}
	for len(out) < t.Rows {
		out = append(out, "")
	}
	// Every row is padded to the full width. A short last row would put the
	// panel's closing zone marker at column 0, leaving the registered zone
	// with EndX < StartX — which bubblezone rejects, so no click or drag in
	// the panel would ever be recognised.
	for i, l := range out {
		if n := t.Cols - lipgloss.Width(l); n > 0 {
			out[i] = l + strings.Repeat(" ", n)
		}
	}
	return out
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
