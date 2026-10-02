package screens

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/ui/theme"
)

// Frame selection: any text on screen can be dragged and copied, the way a
// terminal emulator selects, but inside the app (mouse tracking keeps the host
// terminal from seeing these cells). Panes with their own selection — the
// editors, the terminal panel, the git diff — keep it; a drag that starts
// anywhere else selects a block of the rendered frame and copies it on
// release.
//
// ponytail: block (rectangular) selection only. The frame is many panes side
// by side, so a reading-order selection would sweep the sidebar into every
// copy; a block grabs just the pane under the mouse.
type frameSel struct {
	active         bool // the press turned into a drag
	ax, ay, bx, by int  // anchor and current cell
}

// bounds returns the selected block, ordered.
func (s frameSel) bounds() (x0, y0, x1, y1 int) {
	x0, x1 = min(s.ax, s.bx), max(s.ax, s.bx)
	y0, y1 = min(s.ay, s.by), max(s.ay, s.by)
	return
}

// anchorOwned reports whether the press that started this drag landed in a
// pane that selects for itself.
func (m *MainScreen) anchorOwned() bool {
	p := tea.MouseMsg{X: m.frameSel.ax, Y: m.frameSel.ay}
	in := func(id string) bool {
		z := m.Zones.Get(id)
		return z != nil && !z.IsZero() && z.InBounds(p)
	}
	switch {
	case m.Term.Open && in("term_view"):
		return true
	case m.GitOpen:
		return in("git_panel")
	case m.Palette.Open:
		return in("palette")
	case m.focusedEditor() != nil && in("editor"):
		return true
	}
	return false
}

// OverlaySelection highlights the selected block on a scanned frame and keeps
// the frame's text for the copy. Called by the App after bubblezone's Scan,
// so coordinates are exactly what is on screen.
func (m *MainScreen) OverlaySelection(frame string) string {
	lines := strings.Split(frame, "\n")
	m.lastFrame = lines
	if !m.frameSel.active {
		return frame
	}
	x0, y0, x1, y1 := m.frameSel.bounds()
	for y := y0; y <= y1 && y < len(lines); y++ {
		l := lines[y]
		w := ansi.StringWidth(l)
		if x0 >= w {
			continue
		}
		mid := ansi.Strip(ansi.Cut(l, x0, x1+1))
		if n := x1 + 1 - x0 - ansi.StringWidth(mid); n > 0 && x1 < w {
			mid += strings.Repeat(" ", n) // blank cells still highlight
		}
		lines[y] = ansi.Cut(l, 0, x0) + theme.SelectionStyle.Render(mid) + ansi.Cut(l, x1+1, w)
	}
	return strings.Join(lines, "\n")
}

// frameSelText is the selected block as plain text, one line per row.
func (m *MainScreen) frameSelText() string {
	if !m.frameSel.active {
		return ""
	}
	x0, y0, x1, y1 := m.frameSel.bounds()
	var out []string
	for y := y0; y <= y1 && y < len(m.lastFrame); y++ {
		out = append(out, strings.TrimRight(ansi.Strip(ansi.Cut(m.lastFrame[y], x0, x1+1)), " "))
	}
	return strings.Join(out, "\n")
}

// copyFrameSel puts the block on the clipboard and reports it in the footer.
func (m *MainScreen) copyFrameSel() {
	s := m.frameSelText()
	if strings.TrimSpace(s) == "" {
		m.frameSel.active = false
		return
	}
	if err := clipboard.WriteAll(s); err != nil {
		m.StatusBar = "clipboard unavailable"
		return
	}
	m.StatusBar = fmt.Sprintf("copied %d line(s) · ctrl+v / cmd+v pastes", strings.Count(s, "\n")+1)
}
