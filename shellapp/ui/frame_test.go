package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// A burst of wheel ticks builds one frame, not one per tick: the rest reuse
// it until a single frame tick draws the latest state. A key is never held.
func TestWheelBurstIsCoalesced(t *testing.T) {
	a := NewApp(t.TempDir())
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	first := a.View()
	drawn := a.lastAt

	ticks := 0
	for i := 0; i < 50; i++ {
		_, cmd := a.Update(tea.MouseMsg{X: 60, Y: 20, Type: tea.MouseWheelDown, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
		if cmd != nil {
			ticks++
		}
		if a.View() != first || a.lastAt != drawn {
			t.Fatalf("tick %d built a new frame inside the gap", i)
		}
	}
	if ticks != 1 {
		t.Fatalf("frame ticks scheduled = %d, want 1", ticks)
	}

	a.Update(frameMsg{})
	a.View()
	if !a.lastAt.After(drawn) {
		t.Fatal("the frame tick did not draw")
	}

	time.Sleep(time.Millisecond)
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if a.hold {
		t.Fatal("a key press must draw at once")
	}
}
