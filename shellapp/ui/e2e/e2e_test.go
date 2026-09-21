package e2e

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hinshun/vt10x"
	"github.com/hittable/shellapp/ui"
	"github.com/hittable/shellapp/ui/screens"
)

// harness runs the real bubbletea program with a pipe for input and a
// vt10x emulator on its output, so tests can inject terminal escape
// sequences (mouse clicks) and read the resulting screen.
type harness struct {
	in  io.Writer
	vt  vt10x.Terminal
	p   *tea.Program
	app *ui.App
}

func start(t *testing.T, cols, rows int) (*harness, string) {
	t.Helper()
	tmp := t.TempDir()
	hd := filepath.Join(tmp, "hittable")
	os.MkdirAll(hd, 0o755)
	os.WriteFile(filepath.Join(hd, "env.json"), []byte(`{}`), 0o644)
	os.WriteFile(filepath.Join(hd, "a.hit"), []byte(`{"method":"GET","url":"http://x/a","headers":{},"params":{},"body":"hello world","response":null}`), 0o644)
	os.WriteFile(filepath.Join(hd, "notes.md"), []byte("alpha beta\ngamma delta\n"), 0o644)

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	app := ui.NewApp(tmp)
	p := tea.NewProgram(app, tea.WithInput(inR), tea.WithOutput(outW), tea.WithMouseAllMotion(), tea.WithoutSignals())
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
	time.Sleep(150 * time.Millisecond)
	p.Send(tea.WindowSizeMsg{Width: cols, Height: rows})
	time.Sleep(150 * time.Millisecond)
	t.Cleanup(func() { p.Kill(); inW.Close() })
	return &harness{in: inW, vt: vt, p: p, app: app}, hd
}

func (h *harness) click(x, y int) {
	// SGR mouse: press then release, 1-based coordinates.
	fmt.Fprintf(h.in, "\x1b[<0;%d;%dM\x1b[<0;%d;%dm", x+1, y+1, x+1, y+1)
	time.Sleep(120 * time.Millisecond)
}

func (h *harness) screen() []string {
	h.vt.Lock()
	defer h.vt.Unlock()
	cols, rows := h.vt.Size()
	out := make([]string, rows)
	for y := 0; y < rows; y++ {
		var sb strings.Builder
		for x := 0; x < cols; x++ {
			c := h.vt.Cell(x, y).Char
			if c == 0 {
				c = ' '
			}
			sb.WriteRune(c)
		}
		out[y] = sb.String()
	}
	return out
}

// find returns the terminal (x, y) of the first occurrence of s on screen.
// vt10x stores a double-width emoji in a single cell, so the real column is
// the display width of the prefix, not its cell count.
func (h *harness) find(s string) (int, int, bool) {
	for y, line := range h.screen() {
		if x := strings.Index(line, s); x >= 0 {
			return lipgloss.Width(line[:x]), y, true
		}
	}
	return 0, 0, false
}

func TestMouseThroughRealTerminal(t *testing.T) {
	h, _ := start(t, 120, 40)

	// Expand hittable/ then open notes.md by clicking them in the explorer.
	x, y, ok := h.find("hittable/")
	if !ok {
		t.Fatalf("explorer not rendered:\n%s", strings.Join(h.screen(), "\n"))
	}
	h.click(x, y)
	x, y, ok = h.find("notes.md")
	if !ok {
		t.Fatalf("notes.md not visible after expand:\n%s", strings.Join(h.screen(), "\n"))
	}
	h.click(x, y)
	if _, _, ok = h.find("Ln 1, Col 1"); !ok {
		t.Fatalf("text editor not open:\n%s", strings.Join(h.screen(), "\n"))
	}

	// Click on "delta" (row 2 of the file) and expect the cursor there.
	x, y, ok = h.find("gamma delta")
	if !ok {
		t.Fatal("file content not rendered")
	}
	h.click(x+6, y)
	if _, _, ok = h.find("Ln 2, Col 7"); !ok {
		t.Errorf("click did not move the editor cursor:\n%s", strings.Join(h.screen(), "\n"))
	}

	// Runner: open a.hit, click Body tab, click into "world".
	x, y, _ = h.find("a.hit")
	h.click(x, y)
	x, y, ok = h.find("[Body]")
	if !ok {
		t.Fatalf("runner not open:\n%s", strings.Join(h.screen(), "\n"))
	}
	h.click(x+1, y)
	x, y, ok = h.find("hello world")
	if !ok {
		t.Fatalf("body tab not shown:\n%s", strings.Join(h.screen(), "\n"))
	}
	h.click(x+6, y)
	if _, _, ok = h.find("Ln 1, Col 7"); !ok {
		t.Errorf("click did not move the body cursor:\n%s", strings.Join(h.screen(), "\n"))
	}
}

func TestIntegratedTerminal(t *testing.T) {
	os.Setenv("SHELL", "/bin/sh")
	h, _ := start(t, 120, 40)
	x, y, ok := h.find("TERMINAL")
	if !ok {
		t.Fatalf("terminal strip missing:\n%s", strings.Join(h.screen(), "\n"))
	}
	stripY := y
	h.click(x, y)
	time.Sleep(400 * time.Millisecond)
	if x2, y2, ok := h.find("TERMINAL"); !ok || y2 >= stripY {
		t.Fatalf("strip should move up when the panel opens (was %d, now %d,%d ok=%v):\n%s", stripY, x2, y2, ok, strings.Join(h.screen(), "\n"))
	}
	fmt.Fprint(h.in, "echo e2e_$((6*7))\r")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, ok := h.find("e2e_42"); ok {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, _, ok := h.find("e2e_42"); !ok {
		t.Fatalf("shell output not rendered:\n%s", strings.Join(h.screen(), "\n"))
	}
	if scr := h.screen(); len(scr) != 40 {
		t.Errorf("rows %d", len(scr))
	}
	// Click the strip again while focused: panel closes.
	x, y, _ = h.find("TERMINAL")
	h.click(x, y)
	time.Sleep(200 * time.Millisecond)
	if _, _, ok := h.find("e2e_42"); ok {
		t.Errorf("panel should be hidden after second click")
	}
}

func TestGitButtonInTopBar(t *testing.T) {
	h, _ := start(t, 120, 40)
	x, y, ok := h.find("Git")
	if !ok || y != 0 {
		t.Fatalf("Git button not in top bar:\n%s", strings.Join(h.screen(), "\n"))
	}
	h.click(x, y)
	// The harness directory is not a repository, so the panel explains that.
	if _, _, ok := h.find("Not a git repository"); !ok {
		t.Fatalf("git panel did not open:\n%s", strings.Join(h.screen(), "\n"))
	}
	h.click(x, y)
	if _, _, ok := h.find("Not a git repository"); ok {
		t.Error("second click should close the panel")
	}
}

// ctrl+z (0x1a) through a real terminal undoes typing in the editor.
func TestUndoThroughRealTerminal(t *testing.T) {
	h, _ := start(t, 120, 40)
	x, y, _ := h.find("hittable/")
	h.click(x, y)
	x, y, _ = h.find("notes.md")
	h.click(x, y)
	x, y, ok := h.find("alpha beta")
	if !ok {
		t.Fatal("editor not open")
	}
	h.click(x, y)
	fmt.Fprint(h.in, "ZZZ")
	time.Sleep(150 * time.Millisecond)
	if _, _, ok := h.find("ZZZalpha"); !ok {
		t.Fatalf("typing failed:\n%s", strings.Join(h.screen(), "\n"))
	}
	fmt.Fprint(h.in, "\x1a\x1a\x1a") // ctrl+z x3
	time.Sleep(200 * time.Millisecond)
	if _, _, ok := h.find("ZZZalpha"); ok {
		t.Fatalf("ctrl+z did not undo:\n%s", strings.Join(h.screen(), "\n"))
	}
}
