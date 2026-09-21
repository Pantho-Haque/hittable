package responseviewer

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/hittable/shellapp/ui/theme"
	zone "github.com/lrstanley/bubblezone"
)

func (r *ResponseViewer) highlightJSON(jsonStr string) string {
	lexer := lexers.Get("json")
	if lexer == nil {
		return jsonStr
	}
	lexer = chroma.Coalesce(lexer)
	style := styles.Get("dracula")
	if style == nil {
		style = styles.Fallback
	}
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}
	iterator, err := lexer.Tokenise(nil, jsonStr)
	if err != nil {
		return jsonStr
	}
	var buf strings.Builder
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return jsonStr
	}
	return strings.TrimRight(buf.String(), "\n")
}

// headerRows is the number of rows above the body: status line + blank
// (+ search bar).
func (r *ResponseViewer) headerRows() int {
	if r.SearchOpen {
		return 3
	}
	return 2
}

// bodyRows is the number of body lines that fit in the panel.
func (r *ResponseViewer) bodyRows() int {
	n := r.Height - r.headerRows()
	if n < 1 {
		n = 1
	}
	return n
}

func (r *ResponseViewer) lines() []string {
	if r.ShowHeaders {
		return r.headerLines
	}
	return r.bodyLines
}

func (r *ResponseViewer) View(z *zone.Manager, focused bool) string {
	border := theme.UnfocusedBorderStyle
	if focused {
		border = theme.FocusedBorderStyle
	}
	if r.Empty() {
		return border.Width(r.Width).Height(r.Height).
			Render(theme.MutedStyle.Render(" No response yet · ctrl+r to send"))
	}

	statusStyle := theme.StatusOKStyle
	switch {
	case r.Status >= 500 || r.Status == 0:
		statusStyle = theme.StatusErrorStyle
	case r.Status >= 400:
		statusStyle = theme.StatusErrorStyle
	case r.Status >= 300:
		statusStyle = theme.StatusWarnStyle
	}
	view := "body"
	if r.ShowHeaders {
		view = fmt.Sprintf("headers (%d)", len(r.Headers))
	}
	status := statusStyle.Render(fmt.Sprintf(" %d %s", r.Status, r.StatusText)) +
		theme.MutedStyle.Render(fmt.Sprintf(" · %dms · %s · ", r.DurationMs, humanBytes(r.SizeBytes))) +
		z.Mark("resp_mode", theme.LinkStyle.Render(view))
	rows := []string{z.Mark("search_icon", theme.MutedStyle.Render("🔍")) + " " + status}
	if r.SearchOpen {
		info := ""
		if len(r.SearchMatches) > 0 {
			info = fmt.Sprintf("  %d/%d", r.SearchIdx+1, len(r.SearchMatches))
		} else if r.SearchQuery != "" {
			info = "  no matches"
		}
		rows = append(rows, theme.SearchInputStyle.Render(r.SearchInput.View())+theme.MutedStyle.Render(info))
	}
	rows = append(rows, "")

	all := r.lines()
	start := r.ScrollY
	end := start + r.bodyRows()
	if end > len(all) {
		end = len(all)
	}
	if start > end {
		start = end
	}
	var matches []matchPos
	if r.SearchQuery != "" && !r.ShowHeaders {
		matches = r.findAllMatches(r.rawLines)
	}
	for i := start; i < end; i++ {
		line := all[i]
		if len(matches) > 0 {
			var lm []matchPos
			for _, m := range matches {
				if m.lineIdx == i {
					lm = append(lm, m)
				}
			}
			if len(lm) > 0 {
				line = r.applySearchHighlight(line, r.rawLines[i], lm)
			}
		}
		rows = append(rows, ansi.Truncate(line, r.Width-1, "…"))
	}
	if len(all) > r.bodyRows() {
		pct := 100 * end / len(all)
		rows[0] += theme.MutedStyle.Render(fmt.Sprintf(" · %d%%", pct))
	}
	return border.Width(r.Width).Height(r.Height).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func humanBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%dB", n)
}
