package screens_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/hittable/shellapp/ui/screens"
)

func TestTypeIntoTextEditorAndToggle(t *testing.T) {
	root := "/Users/guess/Desktop/hittable"
	z := zone.New()
	m := screens.NewMainScreen(root, z)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	hitFile := "/Users/guess/Desktop/hittable/hittable/test/test.hit"
	m.OpenFile(hitFile)
	if m.ActiveFile == "" {
		t.Skipf("test.hit not found")
	}

	m.ExplorerFocused = false
	m.Focus = 1 // FocusURLBar

	// Switch to text mode
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	t.Logf("ViewMode=%d", m.ViewMode)

	// Focus text editor (in case toggle didn't auto-focus it)
	m.Focus = 4 // FocusTextEditor
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	type cg interface{ GetContent() string }
	text := interface{}(m.TextEd).(cg).GetContent()
	t.Logf("after typing X: text=%q", text[:min(200, len(text))])

	// Toggle back to runner
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	t.Logf("ViewMode after second Ctrl+T: %d", m.ViewMode)

	// Toggle back to text
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	t.Logf("ViewMode after third Ctrl+T: %d", m.ViewMode)

	text2 := interface{}(m.TextEd).(cg).GetContent()
	t.Logf("after toggle round-trip, text=%q", text2[:min(200, len(text2))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
