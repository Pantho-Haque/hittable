package gitpanel

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/ui/theme"
)

// wrapCells breaks a rendered line into chunks of at most w display cells,
// keeping ANSI styling intact. Always returns at least one chunk.
func wrapCells(s string, w int) []string {
	total := lipgloss.Width(s)
	if w < 1 || total <= w {
		return []string{s}
	}
	var out []string
	for total > w {
		out = append(out, ansi.Cut(s, 0, w))
		s = ansi.Cut(s, w, total)
		total -= w
	}
	return append(out, s)
}

// fullWidth lays a header line across the whole pane, wrapped or clipped.
func fullWidth(s string, width int, wrap bool) []string {
	if wrap {
		return wrapCells(s, width)
	}
	return []string{ansi.Truncate(s, width, "…")}
}

// sideRows renders one diff line as the rows of a single column, each exactly
// half cells wide. With wrap on, a long line continues on further rows with a
// blank number gutter; with it off it is clipped to one row.
func sideRows(n int, text string, st lipgloss.Style, half int, wrap bool) []string {
	vis := visibleWhitespace(text)
	body := half - 5 // the "%4d " gutter
	if body < 1 {
		body = 1
	}
	chunks := []string{ansi.Truncate(vis, body, "…")}
	if wrap {
		chunks = wrapCells(vis, body)
	}
	rows := make([]string, len(chunks))
	for i, c := range chunks {
		s := "     "
		if i == 0 {
			s = fmt.Sprintf("%4d ", n)
		}
		s += c
		if w := lipgloss.Width(s); w < half {
			s += strings.Repeat(" ", half-w)
		}
		rows[i] = st.Render(s)
	}
	return rows
}

// splitDiff turns a unified diff into side-by-side rows (old | new). half is
// the width of the old column, set by dragging the divider. Removed / added
// runs inside a hunk are paired line by line; file headers span the full
// width. With wrap on, a line too long for its column continues on further
// rows instead of being clipped. The second result maps each row to the
// unified-diff line(s) it shows — two for a paired removed / added row.
func splitDiff(unified []string, width, half int, wrap bool, sepSt lipgloss.Style) ([]string, [][]int) {
	var out []string
	var src [][]int
	if half < 8 || width-half-1 < 8 {
		for i, l := range unified {
			out = append(out, l)
			src = append(src, []int{i})
		}
		return out, src
	}
	var dels, adds []string
	var delIdx, addIdx []int
	oldN, newN := 0, 0
	rhalf := width - half - 1
	sep := sepSt.Render("│")
	blank, rblank := strings.Repeat(" ", half), strings.Repeat(" ", rhalf)

	// emit lays one wrapped left column beside one wrapped right column,
	// padding whichever ran out of rows first.
	emit := func(l, r []string, from []int) {
		for i := 0; i < max(len(l), len(r)); i++ {
			lc, rc := blank, rblank
			if i < len(l) {
				lc = l[i]
			}
			if i < len(r) {
				rc = r[i]
			}
			out = append(out, lc+sep+rc)
			src = append(src, from)
		}
	}
	flush := func() {
		for i := 0; i < max(len(dels), len(adds)); i++ {
			var l, r []string
			var from []int
			if i < len(dels) {
				l = sideRows(oldN, dels[i], theme.DiffDelLineStyle, half, wrap)
				from = append(from, delIdx[i])
				oldN++
			}
			if i < len(adds) {
				r = sideRows(newN, adds[i], theme.DiffAddLineStyle, rhalf, wrap)
				from = append(from, addIdx[i])
				newN++
			}
			emit(l, r, from)
		}
		dels, adds, delIdx, addIdx = dels[:0], adds[:0], delIdx[:0], addIdx[:0]
	}
	full := func(rows []string, i int) {
		for _, r := range rows {
			out = append(out, r)
			src = append(src, []int{i})
		}
	}
	plain := lipgloss.NewStyle()
	for i, line := range unified {
		switch {
		case strings.HasPrefix(line, "@@"):
			flush()
			fmt.Sscanf(line, "@@ -%d", &oldN)
			if j := strings.Index(line, "+"); j >= 0 {
				fmt.Sscanf(line[j:], "+%d", &newN)
			}
			full(fullWidth(theme.DiffHunkStyle.Render(line), width, wrap), i)
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			dels, delIdx = append(dels, line[1:]), append(delIdx, i)
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			adds, addIdx = append(adds, line[1:]), append(addIdx, i)
		case strings.HasPrefix(line, " "):
			flush()
			emit(sideRows(oldN, line[1:], plain, half, wrap), sideRows(newN, line[1:], plain, rhalf, wrap), []int{i})
			oldN++
			newN++
		case strings.HasPrefix(line, "\\"):
			flush()
			full([]string{theme.MutedStyle.Render(line)}, i)
		default: // diff/index/---/+++ headers, commit text
			flush()
			full(fullWidth(diffLine(line), width, wrap), i)
		}
	}
	flush()
	return out, src
}
