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
