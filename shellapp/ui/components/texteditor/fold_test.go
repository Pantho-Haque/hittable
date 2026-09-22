package texteditor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const src = `package main

func main() {
	if x {
		do()

		more()
	}
	return
}

var y = 1
`

func TestFoldRegionsByIndent(t *testing.T) {
	lines := strings.Split(src, "\n")
	got := map[int]int{}
	for _, r := range foldRegions(lines) {
		got[r.head] = r.end
	}
	// "func main() {" folds to "return"; "if x {" folds to "more()", with the
	// blank line inside the block rather than ending it.
	want := map[int]int{2: 8, 3: 6}
	if len(got) != len(want) {
		t.Fatalf("regions %v, want %v", got, want)
	}
	for h, e := range want {
		if got[h] != e {
			t.Errorf("region at %d ends at %d, want %d", h, got[h], e)
		}
	}
}

func openEditor(t *testing.T) *TextEditor {
	t.Helper()
	ed := New()
	ed.SetSize(70, 20)
	ed.SetContent("main.go", src)
	ed.Focused = true
	return ed
}

func TestFoldHidesLinesAndShowsASummary(t *testing.T) {
	ed := openEditor(t)
	plain := func() string { return ansi.Strip(ed.View()) }

	if !strings.Contains(plain(), "more()") {
		t.Fatal("body not visible before folding")
	}
	ed.ToggleFold(2) // "func main() {"
	body := plain()
	for _, hidden := range []string{"if x {", "more()", "return"} {
		if strings.Contains(body, hidden) {
			t.Errorf("%q still visible inside a collapsed block:\n%s", hidden, body)
		}
	}
	if !strings.Contains(body, "func main() {") {
		t.Error("the header itself should stay visible")
	}
	if !strings.Contains(body, "⋯") {
		t.Error("no summary shown on the collapsed header")
	}
	// Lines after the block keep their own numbers.
	if !strings.Contains(body, "var y = 1") {
		t.Error("lines after the block should still be visible")
	}

	ed.ToggleFold(2)
	if !strings.Contains(plain(), "more()") {
		t.Error("unfolding did not restore the body")
	}
}

// Folding from inside the block is what the shortcut does most of the time.
func TestFoldFromInsideCollapsesTheEnclosingBlock(t *testing.T) {
	ed := openEditor(t)
	ed.GotoLine(5) // a blank line inside "if x {"
	ed.ToggleFold(ed.GetCursorRow())
	if ed.FoldedCount() != 1 {
		t.Fatalf("expected one folded block, got %d", ed.FoldedCount())
	}
	// The cursor cannot be left inside something that is now hidden.
	if row := ed.GetCursorRow(); row != 3 {
		t.Errorf("cursor left at row %d, want the header at 3", row)
	}
}

// A click on the arrow column toggles; a click on the text does not.
func TestFoldArrowClick(t *testing.T) {
	ed := openEditor(t)
	ed.Update(tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionPress, X: ed.foldCol(), Y: 2})
	if ed.FoldedCount() != 1 {
		t.Fatalf("clicking the arrow did not fold: %d", ed.FoldedCount())
	}
	ed.Update(tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionPress, X: ed.foldCol(), Y: 2})
	if ed.FoldedCount() != 0 {
		t.Errorf("clicking the arrow again did not unfold: %d", ed.FoldedCount())
	}
	ed.Update(tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionPress, X: ed.gutterWidth() + 2, Y: 2})
	if ed.FoldedCount() != 0 {
		t.Error("clicking the text should not fold")
	}
}

// With a block collapsed, a click below it must still land on the right line.
func TestClickMapsThroughAFold(t *testing.T) {
	ed := openEditor(t)
	ed.ToggleFold(2)
	// Visual rows are now 0,1,2 then lines 9,10,11 — the block is hidden, so
	// visual row 5 is "var y = 1" on buffer line 11.
	ed.Update(tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionPress, X: ed.gutterWidth() + 1, Y: 5})
	if row := ed.GetCursorRow(); row != 11 {
		t.Errorf("clicked below a fold and landed on row %d, want 11", row)
	}
}

func TestFoldAllCollapsesEveryBlock(t *testing.T) {
	ed := openEditor(t)
	before := len(strings.Split(ansi.Strip(ed.View()), "\n"))

	ed.FoldAll()
	if ed.FoldedCount() == 0 {
		t.Fatal("FoldAll collapsed nothing")
	}
	body := ansi.Strip(ed.View())
	if !strings.Contains(body, "⋯") {
		t.Fatalf("expected a collapsed placeholder:\n%s", body)
	}
	if !ed.AnyFolded() {
		t.Error("AnyFolded should report the collapsed blocks")
	}

	ed.UnfoldAll()
	if ed.FoldedCount() != 0 {
		t.Fatal("UnfoldAll left blocks collapsed")
	}
	if after := len(strings.Split(ansi.Strip(ed.View()), "\n")); after != before {
		t.Errorf("expanding should restore the original row count: %d, want %d", after, before)
	}
}

func TestToggleFoldAllFlipsBothWays(t *testing.T) {
	ed := openEditor(t)
	ed.ToggleFoldAll()
	if !ed.AnyFolded() {
		t.Fatal("first toggle should collapse")
	}
	ed.ToggleFoldAll()
	if ed.AnyFolded() {
		t.Fatal("second toggle should expand")
	}
}

func TestAltOTogglesFoldAll(t *testing.T) {
	ed := openEditor(t)
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o"), Alt: true})
	if !ed.AnyFolded() {
		t.Fatal("alt+o should collapse every block")
	}
	// macOS terminals send the composed rune instead of a meta-modified key.
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ø")})
	if ed.AnyFolded() {
		t.Fatal("⌥o should expand every block")
	}
}

func TestFoldAllMovesTheCursorOutOfAFold(t *testing.T) {
	ed := openEditor(t)
	ed.GotoLine(3) // inside a block that FoldAll will hide
	ed.FoldAll()

	lines := strings.Split(ed.GetContent(), "\n")
	hidden, _ := ed.cur.fold.hidden(lines)
	if row := ed.GetCursorRow(); row < len(hidden) && hidden[row] {
		t.Fatalf("cursor left on hidden row %d", row)
	}
}

func TestFoldableReportsCollapsibleBlocks(t *testing.T) {
	ed := openEditor(t)
	if !ed.Foldable() {
		t.Error("the fixture has indented blocks and should be foldable")
	}
	ed.SetContent("flat.txt", "one\ntwo\nthree\n")
	if ed.Foldable() {
		t.Error("a file with no indentation has nothing to fold")
	}
}
