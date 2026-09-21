package screens_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	explorerui "github.com/hittable/shellapp/ui/components/explorer"
	"github.com/hittable/shellapp/ui/screens"
)

func TestDoubleClickOpensFile(t *testing.T) {
	// Use the shellapp directory which has predictable structure.
	root := "/Users/guess/Desktop/hittable/shellapp"
	z := zone.New()
	m := screens.NewMainScreen(root, z)

	// Drive a window-size message so dimensions are set.
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	// Find the explorer height by looking at the actual rendered output.
	// The explorer's rowAtY uses (Y - 1) for the first row after the header.
	// But the explorer height is terminal height - 2 (footer takes 1, plus
	// maybe other things). So row 0 of visible = Y=1.
	//
	// Find any file in the visible list and double-click it.
	var fileRow int = -1
	var filePath string
	for i, n := range m.Explorer.Visible {
		if n.Kind != 0 { // skip dirs
			fileRow = i
			filePath = n.Path
			break
		}
	}
	if fileRow < 0 {
		t.Fatalf("no files in visible list")
	}
	t.Logf("clicking file at visible row %d, path=%s", fileRow, filePath)
	y := fileRow + 2 // top bar + explorer header

	_, _ = m.Update(tea.MouseMsg{X: 5, Y: y, Type: tea.MouseLeft, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	_, _ = m.Update(tea.MouseMsg{X: 5, Y: y, Type: tea.MouseLeft, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})

	if m.ActiveFile == "" {
		t.Fatalf("double-click did not open file at row %d", fileRow)
	}

	view := m.View()
	if strings.Contains(strings.ToLower(view), "select a file") {
		t.Errorf("main pane still shows empty placeholder; expected file content. ActiveFile=%s", m.ActiveFile)
	}
	t.Logf("opened: %s", m.ActiveFile)
	t.Logf("view first 500 chars:\n%s", truncate(view, 500))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// We need an unused import suppressor for explorerui — keep here.
var _ = explorerui.MustNew
