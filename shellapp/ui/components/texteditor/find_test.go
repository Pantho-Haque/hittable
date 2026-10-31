package texteditor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestFindHighlightsCountsAndReplaces(t *testing.T) {
	ed := New()
	ed.SetSize(60, 10)
	ed.SetContent("/x/a.txt", "Foo bar foo\nnone\nfoofoo")
	ed.Focus()

	ed.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	keys(ed, "foo")
	if n := len(ed.findMatches()); n != 4 { // case-insensitive, non-overlapping
		t.Fatalf("matches = %d, want 4", n)
	}
	if !strings.Contains(ansi.Strip(ed.View()), "1 of 4") {
		t.Errorf("status lacks the match count:\n%s", ansi.Strip(ed.View()))
	}
	ed.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p := ed.cursorPos(); p != (pos{0, 8}) {
		t.Errorf("next match at %v, want {0 8}", p)
	}
	ed.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p := ed.cursorPos(); p != (pos{0, 0}) {
		t.Errorf("prev match at %v, want {0 0}", p)
	}

	// tab -> replace field; ⏎ replaces the current match and moves on.
	ed.Update(tea.KeyMsg{Type: tea.KeyTab})
	keys(ed, "X")
	ed.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := ed.GetContent(); got != "X bar foo\nnone\nfoofoo" {
		t.Fatalf("replace one: %q", got)
	}
	if p := ed.cursorPos(); p != (pos{0, 6}) {
		t.Errorf("after replace the cursor is at %v, want the next match {0 6}", p)
	}

	ed.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	if got := ed.GetContent(); got != "X bar X\nnone\nXX" {
		t.Fatalf("replace all: %q", got)
	}
	ed.Update(tea.KeyMsg{Type: tea.KeyEsc})
	ed.Update(tea.KeyMsg{Type: tea.KeyCtrlZ}) // replace all is one undo step
	if got := ed.GetContent(); got != "X bar foo\nnone\nfoofoo" {
		t.Errorf("undo replace all: %q", got)
	}
}
