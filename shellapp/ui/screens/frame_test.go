package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// A frame taller than the terminal makes it scroll, which pushes the top bar
// off screen and leaves every mouse coordinate out of step with the layout by
// the number of scrolled rows. Every pane has to fit at every size.
func TestFrameNeverOutgrowsTheTerminal(t *testing.T) {
	sizes := [][2]int{{200, 50}, {120, 40}, {100, 36}, {80, 30}, {70, 25}, {60, 20}, {50, 16}}

	newScreen := func(t *testing.T) (*MainScreen, *zone.Manager, string) {
		tmp := t.TempDir()
		hd := filepath.Join(tmp, "hittable")
		os.MkdirAll(hd, 0o755)
		os.WriteFile(filepath.Join(hd, "env.json"), []byte(`{}`), 0o644)
		md := filepath.Join(hd, "notes.md")
		os.WriteFile(md, []byte(strings.Repeat("a long enough line of prose\n", 40)), 0o644)
		os.WriteFile(filepath.Join(hd, "req.hit"),
			[]byte(`{"method":"GET","url":"http://example.com/a/very/long/path","headers":{},"params":{},"body":"","response":null}`), 0o644)
		z := zone.New()
		m := NewMainScreen(tmp, z)
		return m, z, md
	}

	states := []struct {
		name  string
		setup func(m *MainScreen, file string)
	}{
		{"welcome", func(m *MainScreen, f string) {}},
		{"file open", func(m *MainScreen, f string) { m.openFileRaw(f) }},
		{"help", func(m *MainScreen, f string) { m.ShowHelp = true }},
		{"git panel", func(m *MainScreen, f string) { m.GitOpen = true }},
		{"palette", func(m *MainScreen, f string) { m.Palette.Show(0) }},
		{"terminal open", func(m *MainScreen, f string) { m.Term.Open = true }},
		{"file + terminal", func(m *MainScreen, f string) { m.openFileRaw(f); m.Term.Open = true }},
		{"markdown preview", func(m *MainScreen, f string) { m.openFileRaw(f); m.setMdMode(MdPreview) }},
		{"markdown split", func(m *MainScreen, f string) { m.openFileRaw(f); m.setMdMode(MdSplit) }},
		{"runner", func(m *MainScreen, f string) {
			m.openFileRaw(filepath.Join(filepath.Dir(f), "req.hit"))
		}},
		{"sidebar hidden", func(m *MainScreen, f string) { m.openFileRaw(f); m.ExplorerHidden = true }},
	}

	for _, st := range states {
		for _, sz := range sizes {
			m, z, file := newScreen(t)
			m.SetSize(sz[0], sz[1])
			st.setup(m, file)
			m.SetSize(sz[0], sz[1]) // re-lay out for the new state

			lines := strings.Split(z.Scan(m.View()), "\n")
			if len(lines) != sz[1] {
				t.Errorf("%s at %dx%d: frame is %d rows, terminal has %d",
					st.name, sz[0], sz[1], len(lines), sz[1])
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w > sz[0] {
					t.Errorf("%s at %dx%d: row %d is %d columns wide", st.name, sz[0], sz[1], i, w)
					break
				}
			}
		}
	}
}
