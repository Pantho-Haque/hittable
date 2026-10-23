package screens_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/hittable/shellapp/ui/screens"
)

// A user flips a request to Text mode, types, and flips back and forth:
// the edit must survive every toggle.
func TestTypeIntoTextEditorAndToggle(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "hittable", "test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	hitFile := filepath.Join(dir, "test.hit")
	body := `{"method":"GET","url":"https://example.com/todos/1","headers":{},"params":{},"body":""}`
	if err := os.WriteFile(hitFile, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	m := screens.NewMainScreen(root, zone.New())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.OpenFile(hitFile)
	if m.ActiveFile == "" {
		t.Fatal("test.hit did not open")
	}

	m.ExplorerFocused = false
	m.Focus = screens.FocusURLBar
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	if m.Focus != screens.FocusTextEditor {
		t.Fatalf("ctrl+t should focus the text editor, focus = %d", m.Focus)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})

	type cg interface{ GetContent() string }
	if got := interface{}(m.TextEd).(cg).GetContent(); !strings.Contains(got, "X") {
		t.Fatalf("typed X missing from text editor: %q", got)
	}

	// The stray X breaks the JSON: switching back must refuse, not lose it.
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	if m.Focus != screens.FocusTextEditor || !strings.Contains(m.StatusBar, "Invalid JSON") {
		t.Fatalf("invalid JSON should block the switch, focus=%d status=%q", m.Focus, m.StatusBar)
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT}) // valid again: back to runner
	if m.Focus != screens.FocusURLBar {
		t.Fatalf("valid JSON should switch to runner, focus=%d status=%q", m.Focus, m.StatusBar)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT}) // and to text again
	if got := interface{}(m.TextEd).(cg).GetContent(); !strings.Contains(got, "example.com/todos/1") || strings.Contains(got, "X") {
		t.Fatalf("request changed by the round-trip: %q", got)
	}
}
