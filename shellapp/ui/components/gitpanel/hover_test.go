package gitpanel

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hittable/shellapp/internal/gitx"
)

// The buttons are hit-tested by coordinate, not by zone, so hover and click
// must agree about where they are — that is what rowHit is for.
func TestRowHitAgreesWithHover(t *testing.T) {
	p := New(nil)
	p.Width = 60
	f := gitx.FileStatus{Path: "a.go"}
	p.rows = []row{{file: &f, action: "+", undo: "⟲"}}

	// "[ + ]" sits at the right edge, "[ ⟲ ]" immediately left of it.
	aLeft := p.Width - 2 - 1 - 3
	cases := []struct {
		x    int
		want string
	}{
		{p.Width - 3, "action"},
		{aLeft, "action"},
		{aLeft - 1, "undo"},
		{aLeft - 5, "undo"},
		{aLeft - 6, ""},
		{4, ""},
	}
	for _, c := range cases {
		if got := p.rowHit(&p.rows[0], c.x); got != c.want {
			t.Errorf("rowHit(x=%d) = %q, want %q", c.x, got, c.want)
		}
		// A motion event at the same spot must record the same part.
		p.HandleMouse(tea.MouseMsg{Type: tea.MouseMotion, X: c.x, Y: listTop})
		if p.HoverRow != 0 {
			t.Fatalf("x=%d: hover lost the row (%d)", c.x, p.HoverRow)
		}
		if p.HoverBtn != c.want {
			t.Errorf("hover at x=%d: HoverBtn = %q, want %q", c.x, p.HoverBtn, c.want)
		}
	}

	// Off the list: no row hovered.
	p.HandleMouse(tea.MouseMsg{Type: tea.MouseMotion, X: 4, Y: listTop + p.listRows()})
	if p.HoverRow != -1 {
		t.Fatalf("below the list, HoverRow = %d, want -1", p.HoverRow)
	}
}

// Dragging the divider must move it and stay inside both columns' minimums.
func TestSplitDividerDrag(t *testing.T) {
	p := New(nil)
	p.Width, p.Height = 80, 30
	p.Split = true
	p.detail = []string{"@@ -1,1 +1,1 @@", " ctx"}

	centred := p.splitHalf()
	if centred != (p.detailWidth()-1)/2 {
		t.Fatalf("default divider is not centred: %d", centred)
	}

	press := tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionPress, X: p.dividerX(), Y: p.detailTop()}
	p.HandleMouse(press)
	p.HandleMouse(tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionMotion, X: 25, Y: p.detailTop()})
	if got := p.splitHalf(); got != 24 {
		t.Errorf("after drag, half = %d, want 24", got)
	}

	// Dragged past the edge, both columns keep their minimum width.
	p.HandleMouse(tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionMotion, X: 1, Y: p.detailTop()})
	if got := p.splitHalf(); got < 8 {
		t.Errorf("left column collapsed to %d", got)
	}
	p.HandleMouse(tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionMotion, X: p.Width, Y: p.detailTop()})
	if got := p.detailWidth() - p.splitHalf() - 1; got < 8 {
		t.Errorf("right column collapsed to %d", got)
	}

	// Mouse-up ends the drag; later motion must not keep moving it.
	p.EndDrag()
	before := p.splitHalf()
	p.HandleMouse(tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionMotion, X: 30, Y: p.detailTop()})
	if p.splitHalf() != before {
		t.Error("divider still moving after the drag ended")
	}
}
