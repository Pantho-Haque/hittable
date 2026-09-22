package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
)

const foldableGo = "package main\n\nfunc main() {\n\tprintln(\"hi\")\n\tprintln(\"bye\")\n}\n"

// openSource writes src into the hittable dir and opens it in the editor.
func openSource(t *testing.T, m *MainScreen, hd, name, src string) string {
	t.Helper()
	p := filepath.Join(hd, name)
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m.openFileRaw(p)
	return p
}

func TestFoldAllButtonCollapsesAndRelabels(t *testing.T) {
	m, hd := newTestScreen(t)
	openSource(t, m, hd, "demo.go", foldableGo)

	header := ansi.Strip(m.renderEditorHeader())
	if !strings.Contains(header, "▾ Collapse") {
		t.Fatalf("an unfolded file should offer Collapse:\n%s", header)
	}

	m.TextEd.ToggleFoldAll()
	if m.TextEd.FoldedCount() == 0 {
		t.Fatal("expected every block collapsed")
	}
	if header = ansi.Strip(m.renderEditorHeader()); !strings.Contains(header, "▸ Expand") {
		t.Fatalf("a folded file should offer Expand:\n%s", header)
	}
	// The body hides the folded lines and says what it hid.
	if body := ansi.Strip(m.TextEd.View()); !strings.Contains(body, "⋯") {
		t.Fatalf("collapsed block should show a placeholder:\n%s", body)
	}

	m.TextEd.ToggleFoldAll()
	if m.TextEd.FoldedCount() != 0 {
		t.Fatal("toggling again should expand everything")
	}
	if header = ansi.Strip(m.renderEditorHeader()); !strings.Contains(header, "▾ Collapse") {
		t.Fatalf("label should return to Collapse:\n%s", header)
	}
}

func TestFoldAllButtonClickable(t *testing.T) {
	tmp := t.TempDir()
	hd := filepath.Join(tmp, "hittable")
	os.MkdirAll(hd, 0o755)
	z := zone.New()
	m := NewMainScreen(tmp, z)
	m.SetSize(120, 40)
	openSource(t, m, hd, "demo.go", foldableGo)

	z.Scan(m.View())
	time.Sleep(40 * time.Millisecond)

	fz := z.Get("fold_all")
	if fz == nil || fz.IsZero() {
		t.Fatal("the fold toggle did not register a zone")
	}
	click := tea.MouseMsg{
		Type: tea.MouseLeft, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
		X: (fz.StartX + fz.EndX) / 2, Y: (fz.StartY + fz.EndY) / 2,
	}
	if !fz.InBounds(click) {
		t.Fatal("the middle of the toggle is not inside its own zone")
	}

	m.Update(click)
	if m.TextEd.FoldedCount() == 0 {
		t.Error("clicking the toggle should collapse every block")
	}
	m.Update(click)
	if m.TextEd.FoldedCount() != 0 {
		t.Error("clicking again should expand every block")
	}
}

func TestFoldAllButtonHiddenWithNothingToFold(t *testing.T) {
	m, hd := newTestScreen(t)
	openSource(t, m, hd, "flat.txt", "one\ntwo\nthree\n")

	if header := ansi.Strip(m.renderEditorHeader()); strings.Contains(header, "Collapse") {
		t.Fatalf("a file with no collapsible block should not offer the toggle:\n%s", header)
	}
}

func TestEditorHeaderNeverOutgrowsThePane(t *testing.T) {
	// A header wider than the pane wraps, which scrolls the whole frame and
	// puts every mouse coordinate out of step with the layout.
	for _, w := range []int{50, 60, 72, 90, 120, 160} {
		m, hd := newTestScreen(t)
		m.SetSize(w, 30)
		for _, name := range []string{"demo.go", "notes.md"} {
			src := foldableGo
			if strings.HasSuffix(name, ".md") {
				src = "# Title\n\n- a\n  - nested\n  - deeper\n"
			}
			openSource(t, m, hd, name, src)
			got := lipglossWidth(ansi.Strip(m.renderEditorHeader()))
			if got > m.MainWidth {
				t.Errorf("term %d, %s: header is %d wide, pane is %d", w, name, got, m.MainWidth)
			}
		}
	}
}

// lipglossWidth is the display width of an already-stripped string.
func lipglossWidth(s string) int {
	w := 0
	for _, line := range strings.Split(s, "\n") {
		if n := ansi.StringWidth(line); n > w {
			w = n
		}
	}
	return w
}
