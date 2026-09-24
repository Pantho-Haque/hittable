package terminal

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestShellRoundTrip(t *testing.T) {
	term := New(t.TempDir(), nil)
	term.shell = "/bin/sh"
	term.SetSize(60, 8)
	if err := term.Start(); err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	time.Sleep(200 * time.Millisecond)
	for _, r := range "echo hi_$((40+2))" {
		term.SendKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	term.SendKey(tea.KeyMsg{Type: tea.KeyEnter})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(term.Snapshot(), "hi_42") {
			if v := term.View(); strings.Count(v, "\n") != 7 {
				t.Errorf("View should have 8 rows, got %d", strings.Count(v, "\n")+1)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("shell output not seen:\n%s", term.Snapshot())
}

func TestKeyBytes(t *testing.T) {
	cases := map[string][]byte{
		"ctrl+c": {3}, "enter": []byte("\r"), "up": []byte("\x1b[A"), "backspace": {0x7f},
	}
	for name, want := range cases {
		var msg tea.KeyMsg
		switch name {
		case "ctrl+c":
			msg = tea.KeyMsg{Type: tea.KeyCtrlC}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		}
		if got := keyBytes(msg); string(got) != string(want) {
			t.Errorf("%s: got %q want %q", name, got, want)
		}
	}
}

func TestScrollbackAndPaste(t *testing.T) {
	term := New(t.TempDir(), nil)
	term.shell = "/bin/sh"
	term.SetSize(60, 6)
	if err := term.Start(); err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	time.Sleep(200 * time.Millisecond)
	// Print 30 numbered lines: 24+ must land in scrollback.
	term.Paste("i=1; while [ $i -le 30 ]; do echo line_$i; i=$((i+1)); done\n")
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(term.Snapshot(), "line_30") {
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(term.Snapshot(), "line_30") {
		t.Fatalf("loop did not run:\n%s", term.Snapshot())
	}
	time.Sleep(100 * time.Millisecond)
	if n := term.ScrollbackLen(); n < 20 {
		t.Fatalf("scrollback has %d lines, want >= 20", n)
	}
	term.Scroll(20)
	if v := term.View(); !strings.Contains(v, "line_1") || strings.Contains(v, "line_30") {
		t.Errorf("scrolled view should show older lines:\n%s", v)
	}
	term.SendKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if term.ScrollOffset != 0 {
		t.Error("a key should snap back to the live screen")
	}
	// A large paste must not block the caller.
	done := make(chan struct{})
	go func() { term.Paste(strings.Repeat("# filler text for the paste buffer\n", 400)); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Paste blocked the caller")
	}
}

func TestSelectionText(t *testing.T) {
	// Not running: the view is the placeholder line plus blank rows, which is
	// enough to exercise the row/col maths.
	term := &Terminal{Cols: 20, Rows: 3}

	if term.HasSelection() {
		t.Fatal("fresh terminal reports a selection")
	}
	term.SelectStart(0, 0)
	if term.HasSelection() {
		t.Fatal("zero-width selection counts as one")
	}

	term.SelectTo(8, 0)
	if !term.HasSelection() {
		t.Fatal("drag did not select")
	}
	if got := term.SelectionText(); got != "starting" {
		t.Fatalf("got %q, want %q", got, "starting")
	}

	// Backwards drag selects the same span.
	term.SelectStart(8, 0)
	term.SelectTo(0, 0)
	if got := term.SelectionText(); got != "starting" {
		t.Fatalf("reversed drag: got %q", got)
	}

	// Spanning rows: to the end of row 0, then all of the blank row 1.
	term.SelectStart(0, 0)
	term.SelectTo(0, 1)
	if got := term.SelectionText(); got != "starting shell…\n" {
		t.Fatalf("multi-row: got %q", got)
	}

	term.Scroll(1)
	if term.HasSelection() {
		t.Fatal("scrolling did not drop the selection")
	}
}

func TestHighlight(t *testing.T) {
	got := highlight("abcdef", 2, 4, 6)
	if want := "ab\x1b[7mcd\x1b[27mef"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// Past the end of a short row: blank cells still highlight.
	if got := highlight("ab", 0, 4, 4); got != "\x1b[7mab  \x1b[27m" {
		t.Fatalf("padding: got %q", got)
	}
}

// A child that asks where the cursor is must get an answer. vt10x discards its
// replies unless a writer is wired up (replyWriter), and a TUI that gets no
// answer draws over its own output.
func TestCursorPositionReport(t *testing.T) {
	term := New(t.TempDir(), nil)
	term.shell = "/bin/sh"
	term.SetSize(60, 8)
	if err := term.Start(); err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	time.Sleep(300 * time.Millisecond)

	for _, r := range "printf '\\033[6n'" {
		term.SendKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	term.SendKey(tea.KeyMsg{Type: tea.KeyEnter})

	// The reply (ESC [ row ; col R) arrives on the shell's stdin with no
	// newline, so it sits in the line buffer until we terminate the line. The
	// shell then fails to run "[row;colR", which prints the coordinates as
	// plain text — visible proof the answer came back.
	time.Sleep(500 * time.Millisecond)
	term.SendKey(tea.KeyMsg{Type: tea.KeyEnter})

	want := regexp.MustCompile(`;\d+R`)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if want.MatchString(term.Snapshot()) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("no cursor position report reached the shell; screen:\n%s", term.Snapshot())
}

// vt10x dispatches CSI on the final byte without understanding the private
// prefix, so a kitty-keyboard query reads as "restore cursor". Claude Code
// sends \x1b[?u on every keystroke; unfiltered, each character lands wherever
// the cursor was last saved instead of in the program's input box.
func TestStripUnsupportedCSI(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"kitty query", "a\x1b[?ub", "ab"},
		{"kitty pop", "a\x1b[<ub", "ab"},
		{"kitty push", "a\x1b[>5ub", "ab"},
		{"xtsave", "a\x1b[?1049sb", "ab"},
		{"xtmodkeys", "a\x1b[>4;2mb", "ab"},
		{"xtversion", "a\x1b[>0qb", "ab"},
		{"decset kept", "a\x1b[?2004hb", "a\x1b[?2004hb"},
		{"decrst kept", "a\x1b[?25lb", "a\x1b[?25lb"},
		{"real decrc kept", "a\x1b[ub", "a\x1b[ub"},
		{"real decsc kept", "a\x1b[sb", "a\x1b[sb"},
		{"sgr kept", "a\x1b[38;5;42mb", "a\x1b[38;5;42mb"},
		{"cursor up kept", "a\x1b[4Ab", "a\x1b[4Ab"},
		{"esc7 kept", "a\x1b7b", "a\x1b7b"},
		{"osc kept", "a\x1b]0;title\x07b", "a\x1b]0;title\x07b"},
	} {
		out, carry := stripUnsupportedCSI([]byte(c.in))
		if got := string(out); got != c.want || len(carry) != 0 {
			t.Errorf("%s: got %q carry %q, want %q", c.name, got, carry, c.want)
		}
	}
}

// A sequence split across two reads must not be mistaken for literal text.
func TestStripUnsupportedCSISplit(t *testing.T) {
	full := "x\x1b[?uy"
	for cut := 1; cut < len(full); cut++ {
		out1, carry := stripUnsupportedCSI([]byte(full[:cut]))
		got := string(out1)
		rest := append(append([]byte(nil), carry...), full[cut:]...)
		out2, carry2 := stripUnsupportedCSI(rest)
		got += string(out2)
		if got != "xy" || len(carry2) != 0 {
			t.Errorf("cut at %d: got %q carry %q, want %q", cut, got, carry2, "xy")
		}
	}
}
