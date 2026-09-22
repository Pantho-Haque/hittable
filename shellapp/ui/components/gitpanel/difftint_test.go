package gitpanel

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// Added / removed lines carry a whole-row tint that reaches the right edge,
// so the eye can follow a change across the pane.
func TestDiffRowsAreTintedToTheEdge(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor) // tests have no TTY to detect
	defer lipgloss.SetColorProfile(old)

	p := New(nil)
	p.Width, p.Height = 60, 30
	p.detail = []string{
		"diff --git a/x.go b/x.go",
		"@@ -1,3 +1,3 @@",
		" ctx line",
		"-removed line",
		"+added line",
	}

	width := p.detailWidth()
	for _, mode := range []bool{false, true} { // inline, then split
		p.Split = mode
		p.detailCache = nil
		for _, l := range p.detailLines() {
			plain := ansi.Strip(l)
			body := strings.TrimSpace(plain)
			tinted := strings.Contains(l, "48;2;") // background, in a combined SGR run
			switch {
			case strings.HasPrefix(body, "+added"), strings.HasPrefix(body, "-removed"):
				if !tinted {
					t.Errorf("split=%v: %q has no background", mode, body)
				}
				if w := lipgloss.Width(l); w != width {
					t.Errorf("split=%v: %q is %d wide, want the full %d", mode, body, w, width)
				}
			case strings.HasPrefix(body, "diff --git"), strings.HasPrefix(body, "ctx"):
				if tinted {
					t.Errorf("split=%v: %q should not be tinted", mode, body)
				}
			}
		}
	}
}
