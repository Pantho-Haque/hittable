package texteditor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keys(ed *TextEditor, s string) {
	for _, r := range s {
		ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestUndoRedoTabFindClick(t *testing.T) {
	ed := New()
	ed.SetSize(40, 10)
	ed.SetContent("/x/main.go", "package main\n\nfunc main() {}\n")
	ed.Focus()

	keys(ed, "// hi")
	if !strings.HasPrefix(ed.GetContent(), "// hipackage") {
		t.Fatalf("typing failed: %q", ed.GetContent())
	}
	for i := 0; i < 5; i++ {
		ed.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	}
	if !strings.HasPrefix(ed.GetContent(), "package main") {
		t.Errorf("undo failed: %q", ed.GetContent())
	}
	ed.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	if !strings.HasPrefix(ed.GetContent(), "/package") {
		t.Errorf("redo failed: %q", ed.GetContent())
	}
	ed.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})

	ed.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !strings.HasPrefix(ed.GetContent(), "  package") {
		t.Errorf("tab should insert spaces: %q", ed.GetContent())
	}
	ed.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})

	// find "main()" -> row 2
	ed.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	keys(ed, "main(")
	ed.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if ed.GetCursorRow() != 2 {
		t.Errorf("find: cursor row %d, want 2", ed.GetCursorRow())
	}
	ed.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if ed.PromptOpen() {
		t.Error("esc should close find")
	}

	// go to line 1
	ed.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	keys(ed, "1")
	ed.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if ed.GetCursorRow() != 0 {
		t.Errorf("goto: cursor row %d, want 0", ed.GetCursorRow())
	}

	// click row 2 (relative coords), gutter is 3 wide -> col 5
	ed.Update(tea.MouseMsg{Type: tea.MouseLeft, X: 3 + 5, Y: 2})
	if ed.GetCursorRow() != 2 || ed.TextArea.LineInfo().ColumnOffset != 5 {
		t.Errorf("click: row %d col %d", ed.GetCursorRow(), ed.TextArea.LineInfo().ColumnOffset)
	}

	view := ed.View()
	if !strings.Contains(view, "\x1b[") || !strings.Contains(view, " 3 ") {
		t.Errorf("view should be highlighted with line numbers:\n%s", view)
	}
	if !strings.Contains(view, "Ln 3, Col 6") {
		t.Errorf("status row wrong:\n%s", view)
	}
}
