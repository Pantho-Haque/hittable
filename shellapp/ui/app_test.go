package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The frame the program actually writes (scanned, overlaid, clipped) never
// outgrows the terminal, down to sizes no layout fits, and never panics.
func TestAppViewFitsAnySize(t *testing.T) {
	tmp := t.TempDir()
	hd := filepath.Join(tmp, "hittable")
	os.MkdirAll(hd, 0o755)
	os.WriteFile(filepath.Join(hd, "a.hit"), []byte(`{"method":"GET","url":"http://x"}`), 0o644)
	app := NewApp(tmp)
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {3, 2}, {12, 4}, {40, 6}, {200, 3}, {80, 24}} {
		app.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		frame := app.View()
		frame = frame[strings.Index(frame, "\x1b\\")+2:] // drop the OSC 22 pointer prefix
		if sz[0] == 0 || sz[1] == 0 {
			continue // nothing to fit; only "no panic" applies
		}
		lines := strings.Split(frame, "\n")
		if len(lines) > sz[1] {
			t.Errorf("%v: %d rows", sz, len(lines))
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > sz[0] {
				t.Errorf("%v: row %d is %d wide", sz, i, w)
			}
		}
	}
}
