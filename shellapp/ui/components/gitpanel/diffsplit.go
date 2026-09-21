package gitpanel

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/ui/theme"
)

// splitDiff turns a unified diff into side-by-side rows (old | new), each
// rendered to exactly width cells. Removed / added runs inside a hunk are
// paired line by line; file headers span the full width.
func splitDiff(unified []string, width int) []string {
	half := (width - 1) / 2
	if half < 8 {
		return unified
	}
	var out []string
	var dels, adds []string
	oldN, newN := 0, 0
	side := func(n int, text string, st lipgloss.Style, blank bool) string {
		if blank {
			return strings.Repeat(" ", half)
		}
		s := fmt.Sprintf("%4d %s", n, strings.ReplaceAll(text, "\t", "    "))
		s = ansi.Truncate(s, half, "…")
		if w := lipgloss.Width(s); w < half {
			s += strings.Repeat(" ", half-w)
		}
		return st.Render(s)
	}
	sep := theme.MutedStyle.Render("│")
	flush := func() {
		n := len(dels)
		if len(adds) > n {
			n = len(adds)
		}
		for i := 0; i < n; i++ {
			l, r := strings.Repeat(" ", half), strings.Repeat(" ", half)
			if i < len(dels) {
				l = side(oldN, dels[i], theme.DiffDelStyle, false)
				oldN++
			}
			if i < len(adds) {
				r = side(newN, adds[i], theme.DiffAddStyle, false)
				newN++
			}
			out = append(out, l+sep+r)
		}
		dels, adds = dels[:0], adds[:0]
	}
	plain := lipgloss.NewStyle()
	for _, line := range unified {
		switch {
		case strings.HasPrefix(line, "@@"):
			flush()
			fmt.Sscanf(line, "@@ -%d", &oldN)
			if i := strings.Index(line, "+"); i >= 0 {
				fmt.Sscanf(line[i:], "+%d", &newN)
			}
			out = append(out, ansi.Truncate(theme.DiffHunkStyle.Render(line), width, "…"))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			dels = append(dels, line[1:])
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			adds = append(adds, line[1:])
		case strings.HasPrefix(line, " "):
			flush()
			out = append(out, side(oldN, line[1:], plain, false)+sep+side(newN, line[1:], plain, false))
			oldN++
			newN++
		case strings.HasPrefix(line, "\\"):
			flush()
			out = append(out, theme.MutedStyle.Render(line))
		default: // diff/index/---/+++ headers, commit text
			flush()
			out = append(out, ansi.Truncate(diffLine(line), width, "…"))
		}
	}
	flush()
	return out
}
