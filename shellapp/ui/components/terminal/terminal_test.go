package terminal

import (
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
