package screens

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// The Git panel and the find palette take the whole width; the file tree
// comes back untouched when they close.
func TestFullWidthPanelsHideTheSidebar(t *testing.T) {
	z := zone.New()
	m := NewMainScreen(t.TempDir(), z)
	m.SetSize(160, 45)

	full := m.Width
	docked := m.MainWidth
	if docked >= full {
		t.Fatalf("sidebar is not taking any width (main %d of %d)", docked, full)
	}
	if m.explorerHidden() {
		t.Fatal("sidebar starts hidden")
	}

	cases := []struct {
		name  string
		open  func()
		close func()
	}{
		{"git", m.toggleGit, m.toggleGit},
		{"palette", func() { m.openPalette(0) }, m.Palette.Close},
	}
	for _, c := range cases {
		c.open()
		m.Update(nil) // the layout is recomputed at the end of Update
		if !m.explorerHidden() {
			t.Errorf("%s: sidebar still shown", c.name)
		}
		if m.MainWidth != full {
			t.Errorf("%s: pane is %d wide, want the full %d", c.name, m.MainWidth, full)
		}
		if w := lipgloss.Width(z.Scan(m.View())); w != full {
			t.Errorf("%s: rendered %d columns, want %d", c.name, w, full)
		}

		c.close()
		m.Update(nil)
		if m.explorerHidden() {
			t.Errorf("%s: sidebar did not come back", c.name)
		}
		if m.MainWidth != docked {
			t.Errorf("%s: pane is %d wide after closing, want %d", c.name, m.MainWidth, docked)
		}
	}

	// A sidebar the user hid stays hidden after a panel closes.
	m.toggleExplorer()
	m.toggleGit()
	m.Update(nil)
	m.toggleGit()
	m.Update(nil)
	if !m.explorerHidden() {
		t.Error("a manually hidden sidebar was brought back by closing the panel")
	}
}
