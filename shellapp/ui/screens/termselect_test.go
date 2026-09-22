package screens

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

// Dragging over the terminal panel has to reach the terminal even when a file
// is open: focusTerminal leaves m.Focus on the editor, so the drag used to be
// handed to the editor and no selection was ever made.
func TestTerminalDragSelectsWithAFileOpen(t *testing.T) {
	tmp := t.TempDir()
	hd := filepath.Join(tmp, "hittable")
	os.MkdirAll(hd, 0o755)
	os.WriteFile(filepath.Join(hd, "env.json"), []byte(`{}`), 0o644)
	md := filepath.Join(hd, "notes.md")
	os.WriteFile(md, []byte("alpha beta\ngamma delta\n"), 0o644)

	z := zone.New()
	m := NewMainScreen(tmp, z)
	m.SetSize(120, 40)
	m.openFileRaw(md)
	m.Term.Open = true
	m.SetSize(120, 40)
	m.focusTerminal()

	if m.focusedEditor() != nil {
		t.Fatal("the editor still claims focus while the terminal has it")
	}

	// Register the zones, then drag across the terminal panel.
	z.Scan(m.View())
	time.Sleep(40 * time.Millisecond) // zones register asynchronously
	tz := z.Get("term_view")
	if tz == nil || tz.IsZero() {
		t.Fatal("terminal panel did not register a zone")
	}
	// Row 0 of the panel carries text even before the shell is up.
	x, y := tz.StartX, tz.StartY
	m.handleMouse(tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: x, Y: y})
	m.handleMouse(tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: x + 8, Y: y})

	if !m.Term.HasSelection() {
		t.Fatal("dragging over the terminal made no selection")
	}
	if got := m.Term.SelectionText(); got != "starting" {
		t.Errorf("selected %q, want %q", got, "starting")
	}
}

// The panel's zone has to be a usable rectangle. Short rows would put the
// closing marker at column 0, giving EndX < StartX, which bubblezone rejects —
// no click or drag in the terminal would register at all.
func TestTerminalZoneIsClickable(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "hittable"), 0o755)
	z := zone.New()
	m := NewMainScreen(tmp, z)
	m.Term.Open = true
	m.SetSize(120, 40)
	z.Scan(m.View())
	time.Sleep(40 * time.Millisecond)

	tz := z.Get("term_view")
	if tz == nil || tz.IsZero() {
		t.Fatal("terminal panel did not register a zone")
	}
	if tz.StartX > tz.EndX || tz.StartY > tz.EndY {
		t.Fatalf("zone is degenerate: (%d,%d)-(%d,%d)", tz.StartX, tz.StartY, tz.EndX, tz.EndY)
	}
	mid := tea.MouseMsg{Type: tea.MouseLeft, X: (tz.StartX + tz.EndX) / 2, Y: (tz.StartY + tz.EndY) / 2}
	if !tz.InBounds(mid) {
		t.Error("the middle of the panel is not inside its own zone")
	}
}
