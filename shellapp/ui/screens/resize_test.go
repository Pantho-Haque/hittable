package screens

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
)

func scan(m *MainScreen, z *zone.Manager) {
	z.Scan(m.View())
	time.Sleep(40 * time.Millisecond) // zones register asynchronously
}

func press(x, y int) tea.MouseMsg {
	return tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: x, Y: y}
}
func moveTo(x, y int) tea.MouseMsg {
	return tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: x, Y: y}
}
func release(x, y int) tea.MouseMsg {
	return tea.MouseMsg{Type: tea.MouseRelease, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft, X: x, Y: y}
}

// Dragging the TERMINAL strip resizes the panel; a press that never moves
// still toggles it, as the click always did.
func TestTerminalStripDragAndClick(t *testing.T) {
	m, _ := newTestScreen(t)
	z := m.Zones
	m.SetSize(120, 40)
	m.Term.Open = true
	m.SetSize(120, 40)
	before := m.Term.Rows

	scan(m, z)
	sz := z.Get("term_strip")
	if sz == nil || sz.IsZero() {
		t.Fatal("no term_strip zone")
	}
	m.handleMouse(press(sz.StartX+3, sz.StartY))
	m.handleMouse(moveTo(sz.StartX+3, sz.StartY-6))
	m.handleMouse(release(sz.StartX+3, sz.StartY-6))
	if m.Term.Rows != before+6 || !m.Term.Open {
		t.Fatalf("terminal rows after drag = %d, want %d (open=%v)", m.Term.Rows, before+6, m.Term.Open)
	}
	if m.MainH+1+m.Term.Rows+2 != 40 {
		t.Errorf("layout no longer adds up: main %d + term %d", m.MainH, m.Term.Rows)
	}

	scan(m, z)
	sz = z.Get("term_strip")
	m.handleMouse(press(sz.StartX+3, sz.StartY))
	m.handleMouse(release(sz.StartX+3, sz.StartY))
	if !m.TermFocused {
		t.Fatal("a still click on the strip should focus the open terminal")
	}
	m.handleMouse(press(sz.StartX+3, sz.StartY))
	m.handleMouse(release(sz.StartX+3, sz.StartY))
	if m.Term.Open {
		t.Error("a second still click should hide it")
	}
}

// The border between the runner's editor and response boxes drags.
func TestRunnerSplitterDrag(t *testing.T) {
	m, hd := newTestScreen(t)
	z := m.Zones
	m.SetSize(120, 40)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	before := m.EditorHeight

	scan(m, z)
	ez := z.Get("editor")
	if ez == nil || ez.IsZero() {
		t.Fatal("no editor zone")
	}
	m.handleMouse(press(ez.StartX+5, ez.EndY))
	m.handleMouse(moveTo(ez.StartX+5, ez.EndY+4))
	m.handleMouse(release(ez.StartX+5, ez.EndY+4))
	if m.EditorHeight != before+4 {
		t.Fatalf("editor height after drag = %d, want %d", m.EditorHeight, before+4)
	}
}

// The sidebar is left with ctrl+b (or tab), never with an arrow key.
func TestRightArrowStaysInExplorer(t *testing.T) {
	m, hd := newTestScreen(t)
	m.SetSize(120, 40)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	m.toggleExplorerFocus()
	if !m.ExplorerFocused {
		t.Fatal("explorer should be focused")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRight})
	if !m.ExplorerFocused {
		t.Fatal("→ must not leave the explorer")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	if m.ExplorerFocused {
		t.Fatal("ctrl+b should leave the explorer")
	}
}

// Dragging over text that has no selection of its own — the explorer here —
// selects a block of the frame and copies it on release; a drag that starts in
// the editor still belongs to the editor.
func TestFrameSelectionCopiesAnyVisibleText(t *testing.T) {
	m, hd := newTestScreen(t)
	z := m.Zones
	m.SetSize(120, 40)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	z.Scan(m.View())
	m.OverlaySelection(z.Scan(m.View()))
	row := -1
	for i, l := range m.lastFrame {
		if strings.Contains(l, "a.hit") {
			row = i
			break
		}
	}
	if row < 0 {
		t.Fatal("a.hit not on screen")
	}
	plain := ansi.Strip(m.lastFrame[row])
	col := ansi.StringWidth(plain[:strings.Index(plain, "a.hit")]) // cells, not bytes: icons are wide

	m.handleMouse(press(col, row))
	m.handleMouse(moveTo(col+4, row+1))
	if !m.frameSel.active {
		t.Fatal("drag over the explorer should select the frame")
	}
	got := m.frameSelText()
	if !strings.Contains(got, "a.hit") || strings.Count(got, "\n") != 1 {
		t.Fatalf("selected %q", got)
	}
	out := m.OverlaySelection(z.Scan(m.View()))
	if !strings.Contains(out, "a.hit") || len(strings.Split(out, "\n")) != 40 {
		t.Fatal("overlay broke the frame")
	}
	m.handleMouse(release(col+4, row+1))
	if !strings.HasPrefix(m.StatusBar, "copied 2 line(s)") && m.StatusBar != "clipboard unavailable" {
		t.Errorf("status = %q", m.StatusBar)
	}
	// Any key drops the highlight.
	m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.frameSel.active {
		t.Error("key should clear the frame selection")
	}

	// A drag anchored in the focused editor is the editor's.
	time.Sleep(40 * time.Millisecond)
	ez := z.Get("editor")
	m.handleMouse(press(ez.StartX+3, ez.StartY+1))
	m.handleMouse(moveTo(ez.StartX+8, ez.StartY+1))
	if m.frameSel.active {
		t.Error("drag inside the editor must not become a frame selection")
	}
}
