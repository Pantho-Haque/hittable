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

	// Click row 2, five columns into the text. The gutter width is asked for
	// rather than assumed, so adding a column to it cannot silently break this.
	ed.Update(tea.MouseMsg{Type: tea.MouseLeft, X: ed.gutterWidth() + 5, Y: 2})
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

func TestSoftWrap(t *testing.T) {
	ed := New()
	ed.Wrap = true
	ed.SetSize(30, 8)
	long := "one two three four five six seven eight nine ten eleven twelve"
	ed.SetContent("/x/doc.md", long+"\nshort\n")
	ed.Focus()
	v := stripANSI(ed.View())
	if strings.Contains(v, "…") || !strings.Contains(v, "seven") || !strings.Contains(v, "twelve") {
		t.Fatalf("wrapped view should show the whole line:\n%s", v)
	}
	if strings.Count(v, "\n") != 9 { // Height + 2 border rows
		t.Errorf("view must stay %d rows, got %d", 10, strings.Count(v, "\n")+1)
	}
	if strings.Contains(v, "workf") || strings.Contains(strings.ReplaceAll(v, " ", ""), "sevenseven") {
		t.Errorf("wrapped rows must not repeat text at the break:\n%s", v)
	}
	// Click on the second visual row (continuation) maps into the long line.
	ed.Update(tea.MouseMsg{Type: tea.MouseLeft, X: 3 + 2, Y: 1})
	if ed.GetCursorRow() != 0 || ed.TextArea.LineInfo().ColumnOffset <= 20 {
		t.Errorf("click on continuation row: row=%d col=%d", ed.GetCursorRow(), ed.TextArea.LineInfo().ColumnOffset)
	}
	// End of the long line keeps the cursor visible on a later visual row.
	ed.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if !strings.Contains(stripANSI(ed.View()), "twelve") {
		t.Error("cursor row should be visible")
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			in = true
		case in && (r == 'm' || r == 'z'):
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
