package texteditor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// openCompleter returns a focused editor holding src, sized so the popup has
// room to render below the cursor.
func openCompleter(t *testing.T, path, src string) *TextEditor {
	t.Helper()
	ed := New()
	ed.SetSize(70, 20)
	ed.Focus()
	ed.SetContent(path, src)
	return ed
}

// typeRunes feeds s to the editor one rune at a time, the way a terminal does.
func typeRunes(ed *TextEditor, s string) {
	for _, r := range s {
		ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestCompletionOpensOnTypingAndInsertsOnTab(t *testing.T) {
	ed := openCompleter(t, "main.go", "func calculateTotal() int { return 0 }\n\n")
	ed.GotoLine(1)

	typeRunes(ed, "calc")
	if !ed.CompletionOpen() {
		t.Fatal("typing a 4-rune prefix should have opened the suggestion popup")
	}
	if got := ed.comp.items; len(got) == 0 || got[0] != "calculateTotal" {
		t.Fatalf("expected calculateTotal first, got %v", got)
	}

	body := ansi.Strip(ed.View())
	if !strings.Contains(body, "calculateTotal") {
		t.Fatalf("popup should render the suggestion:\n%s", body)
	}

	ed.Update(tea.KeyMsg{Type: tea.KeyTab})
	if ed.CompletionOpen() {
		t.Fatal("accepting should close the popup")
	}
	line := strings.Split(ed.GetContent(), "\n")[1]
	if line != "calculateTotal" {
		t.Fatalf("expected the prefix replaced by the full word, got %q", line)
	}
}

func TestCompletionStaysClosedBelowMinPrefix(t *testing.T) {
	ed := openCompleter(t, "main.go", "func calculateTotal() int { return 0 }\n\n")
	ed.GotoLine(1)

	typeRunes(ed, "c")
	if ed.CompletionOpen() {
		t.Fatal("a single rune is below completeMinPrefix and must not open the popup")
	}
	// ctrl+space overrides the minimum.
	ed.Update(tea.KeyMsg{Type: tea.KeyCtrlAt})
	if !ed.CompletionOpen() {
		t.Fatal("ctrl+space should offer suggestions regardless of prefix length")
	}
}

func TestCompletionAcceptIsOneUndoStep(t *testing.T) {
	ed := openCompleter(t, "main.go", "func calculateTotal() int { return 0 }\n\n")
	ed.GotoLine(1)

	typeRunes(ed, "calc")
	ed.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := strings.Split(ed.GetContent(), "\n")[1]; got != "calculateTotal" {
		t.Fatalf("setup: expected inserted word, got %q", got)
	}

	ed.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if got := strings.Split(ed.GetContent(), "\n")[1]; got != "calc" {
		t.Fatalf("one undo should roll back only the insertion, got %q", got)
	}
}

func TestCompletionEscapeDismissesWithoutClosingFile(t *testing.T) {
	ed := openCompleter(t, "main.go", "func calculateTotal() int { return 0 }\n\n")
	ed.GotoLine(1)

	typeRunes(ed, "calc")
	if !ed.PromptOpen() {
		t.Fatal("PromptOpen must report the popup, or the screen steals esc to close the file")
	}
	ed.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if ed.CompletionOpen() {
		t.Fatal("esc should dismiss the popup")
	}
	if got := strings.Split(ed.GetContent(), "\n")[1]; got != "calc" {
		t.Fatalf("esc must not alter the text, got %q", got)
	}
}

func TestCompletionNavigatesWithArrows(t *testing.T) {
	ed := openCompleter(t, "main.go", "var alphaOne, alphaTwo, alphaThree int\n\n")
	ed.GotoLine(1)

	typeRunes(ed, "alpha")
	if len(ed.comp.items) < 3 {
		t.Fatalf("expected the three alpha identifiers, got %v", ed.comp.items)
	}
	first := ed.comp.items[ed.comp.idx]
	ed.Update(tea.KeyMsg{Type: tea.KeyDown})
	if second := ed.comp.items[ed.comp.idx]; second == first {
		t.Fatal("down should move the highlight")
	}
	ed.Update(tea.KeyMsg{Type: tea.KeyUp})
	if back := ed.comp.items[ed.comp.idx]; back != first {
		t.Fatalf("up should return to %q, got %q", first, back)
	}
}

func TestHitFileCompletesSchemaKeysAndMethods(t *testing.T) {
	const src = `{
  "method": "",
  "url": "",
  "headers": {
    "": ""
  }
}`
	ed := openCompleter(t, "req.hit", src)

	// Top level: the request schema keys.
	lines := strings.Split(src, "\n")
	if got := ed.schemaCandidates(lines, 6); got[0] != "body" {
		t.Fatalf("top level should offer the schema keys, got %v", got)
	}
	// On the "method" line: HTTP verbs.
	if got := ed.schemaCandidates(lines, 1); got[0] != "DELETE" {
		t.Fatalf("method line should offer HTTP verbs, got %v", got)
	}
	// Inside the headers object: header names.
	if got := ed.schemaCandidates(lines, 4); got[0] != "Accept" {
		t.Fatalf("headers block should offer header names, got %v", got)
	}
}

func TestSchemaCandidatesOnlyForHitFiles(t *testing.T) {
	ed := openCompleter(t, "notes.txt", "{\n  \"method\": \"\"\n}")
	if got := ed.schemaCandidates(strings.Split(ed.GetContent(), "\n"), 1); got != nil {
		t.Fatalf("a non-.hit file must not get request-schema suggestions, got %v", got)
	}
}

func TestCompletionRanksSchemaAboveBufferWords(t *testing.T) {
	// "methodical" is in the buffer, but "method" is part of the schema and
	// should be offered first.
	ed := openCompleter(t, "req.hit", "{\n  \"note\": \"methodical\",\n  \n}")
	ed.GotoLine(2)
	typeRunes(ed, "  met")

	if !ed.CompletionOpen() {
		t.Fatal("expected suggestions for the prefix 'met'")
	}
	items := ed.comp.items
	if items[0] != "method" {
		t.Fatalf("schema key should rank first, got %v", items)
	}
	if len(items) < 2 || items[1] != "methodical" {
		t.Fatalf("buffer word should still be offered after the schema key, got %v", items)
	}
}

func TestCompletionClosesWhenNothingMatches(t *testing.T) {
	ed := openCompleter(t, "main.go", "func calculateTotal() int { return 0 }\n\n")
	ed.GotoLine(1)

	typeRunes(ed, "calc")
	if !ed.CompletionOpen() {
		t.Fatal("setup: popup should be open")
	}
	typeRunes(ed, "zzz")
	if ed.CompletionOpen() {
		t.Fatal("a prefix matching nothing should close the popup")
	}
}

func TestCompletionOffersGoKeywords(t *testing.T) {
	ed := openCompleter(t, "main.go", "\n")
	typeRunes(ed, "ret")
	if !ed.CompletionOpen() {
		t.Fatal("keywords should be offered in an otherwise empty file")
	}
	if got := ed.comp.items[0]; got != "return" {
		t.Fatalf("expected the go keyword return, got %q", got)
	}
}

func TestCompletionClearedBySwitchingFiles(t *testing.T) {
	ed := openCompleter(t, "main.go", "func calculateTotal() int { return 0 }\n\n")
	ed.GotoLine(1)
	typeRunes(ed, "calc")
	if !ed.CompletionOpen() {
		t.Fatal("setup: popup should be open")
	}

	ed.SetContent("other.go", "package main\n")
	if ed.CompletionOpen() {
		t.Fatal("opening another file must dismiss the popup")
	}
}

// runeCol returns the column of r in s, or -1. Byte offsets are useless here
// because the frame is drawn with multi-byte box characters.
func runeCol(s string, r rune) int {
	for i, c := range []rune(s) {
		if c == r {
			return i
		}
	}
	return -1
}

func TestCompletionPopupStaysAlignedOverBlankRows(t *testing.T) {
	// The list is long enough to spill onto the blank rows below the last line
	// of the file. Those rows are empty strings in the frame, and cutting an
	// empty string yields nothing — so the box used to slide to column 0.
	ed := openCompleter(t, "req.hit", "{\n  \"headers\": {\n    \n  }\n}\n")
	ed.SetSize(64, 14)
	ed.GotoLine(2)
	typeRunes(ed, "    \"Con")

	if !ed.CompletionOpen() {
		t.Fatal("expected header suggestions inside the headers block")
	}

	var top, bottom int = -1, -1
	for i, line := range strings.Split(ansi.Strip(ed.View()), "\n") {
		if i == 0 {
			continue // the editor's own border
		}
		if c := runeCol(line, '╭'); c > 0 {
			top = c
		}
		if c := runeCol(line, '╰'); c > 0 {
			bottom = c
		}
	}

	if top <= 0 {
		t.Fatalf("popup top border not found indented into the frame (col %d)", top)
	}
	if bottom != top {
		t.Fatalf("popup bottom border at col %d, top at col %d: the box is not aligned", bottom, top)
	}
}
