package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// One wheel tick as the program runs it: Update, then the whole frame
// (View, the zone scan, the selection overlay).
func wheelTick(m *MainScreen, x, y int, down bool) {
	t := tea.MouseWheelUp
	if down {
		t = tea.MouseWheelDown
	}
	m.Update(tea.MouseMsg{X: x, Y: y, Type: t, Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	m.OverlaySelection(m.Zones.Scan(m.View()))
}

func bigGo(n int) string {
	var b strings.Builder
	b.WriteString("package big\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "func f%d(a, b int) int {\n\tif a > b { return a - b } // comment %d\n\treturn b + a*%d\n}\n\n", i, i, i)
	}
	return b.String()
}

func BenchmarkWheelEditor(b *testing.B) {
	m, hd := newTestScreen(b)
	p := filepath.Join(hd, "big.go")
	os.WriteFile(p, []byte(bigGo(800)), 0o644)
	m.openFileRaw(p)
	m.OverlaySelection(m.Zones.Scan(m.View()))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wheelTick(m, 80, 20, (i/200)%2 == 0)
	}
}

func BenchmarkWheelExplorer(b *testing.B) {
	m, hd := newTestScreen(b)
	for i := 0; i < 300; i++ {
		os.WriteFile(filepath.Join(hd, fmt.Sprintf("f%03d.md", i)), []byte("x\n"), 0o644)
	}
	m.Explorer.RebuildTree()
	m.OverlaySelection(m.Zones.Scan(m.View()))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wheelTick(m, 5, 20, (i/100)%2 == 0)
	}
}

func BenchmarkWheelEditorUpdateOnly(b *testing.B) {
	m, hd := newTestScreen(b)
	p := filepath.Join(hd, "big.go")
	os.WriteFile(p, []byte(bigGo(800)), 0o644)
	m.openFileRaw(p)
	m.OverlaySelection(m.Zones.Scan(m.View()))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t := tea.MouseWheelUp
		if (i/200)%2 == 0 {
			t = tea.MouseWheelDown
		}
		m.Update(tea.MouseMsg{X: 80, Y: 20, Type: t, Action: tea.MouseActionPress})
	}
}
