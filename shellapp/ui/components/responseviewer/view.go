package responseviewer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/hittable/shellapp/ui/theme"
	zone "github.com/lrstanley/bubblezone"
)

func (r *ResponseViewer) highlightJSON(jsonStr string) string {
	lexer := lexers.Get("json")
	if lexer == nil {
		return jsonStr
	}
	lexer = chroma.Coalesce(lexer)
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}
	formatter := formatters.Get("terminal")
	if formatter == nil {
		formatter = formatters.Fallback
	}
	iterator, err := lexer.Tokenise(nil, jsonStr)
	if err != nil {
		return jsonStr
	}
	var buf strings.Builder
	formatter.Format(&buf, style, iterator)
	return buf.String()
}

func (r *ResponseViewer) View(z *zone.Manager) string {
	if r.Content == "" && r.Status == 0 {
		return theme.UnfocusedBorderStyle.Width(r.Width).Height(r.Height).
			Render(lipgloss.NewStyle().Foreground(theme.MutedColor).Render("No response yet. Press Ctrl+R to send."))
	}

	var lines []string

	statusStyle := theme.StatusOKStyle
	if !r.Ok {
		statusStyle = theme.StatusErrorStyle
	}
	searchIcon := z.Mark("search_icon", theme.MutedStyle.Render("🔍"))
	statusLine := statusStyle.Render(fmt.Sprintf(" Response · %d %s · %dms · %dB", r.Status, r.StatusText, r.DurationMs, r.SizeBytes))
	lines = append(lines, searchIcon+" "+statusLine)
	lines = append(lines, "")

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(r.Content), "", "  "); err != nil {
		lines = append(lines, r.Content)
	} else {
		highlighted := r.highlightJSON(prettyJSON.String())
		lines = append(lines, highlighted)
	}

	content := strings.Join(lines, "\n")
	contentLines := strings.Split(content, "\n")
	rawLines := strings.Split(r.RawContent, "\n")

	maxLines := r.Height - 4
	if r.SearchOpen {
		maxLines -= 2
	}
	if maxLines < 1 {
		maxLines = 1
	}

	start := r.ScrollY
	if start > len(contentLines)-maxLines {
		start = len(contentLines) - maxLines
	}
	if start < 0 {
		start = 0
	}
	end := start + maxLines
	if end > len(contentLines) {
		end = len(contentLines)
	}

	allMatches := r.findAllMatches(rawLines)

	visibleLines := contentLines[start:end]
	var result []string
	for i, line := range visibleLines {
		rawIdx := start + i - 2
		if rawIdx >= 0 && rawIdx < len(rawLines) {
			var lineMatches []matchPos
			for _, m := range allMatches {
				if m.lineIdx == rawIdx {
					lineMatches = append(lineMatches, m)
				}
			}
			result = append(result, r.applySearchHighlight(line, rawLines[rawIdx], lineMatches))
		} else {
			result = append(result, line)
		}
	}

	visibleContent := strings.Join(result, "\n")

	if r.SearchOpen {
		searchInfo := ""
		if len(r.SearchMatches) > 0 {
			searchInfo = fmt.Sprintf(" %d/%d ", r.SearchIdx+1, len(r.SearchMatches))
		} else if r.SearchQuery != "" {
			searchInfo = " 0 results "
		}
		searchBar := theme.SearchInputStyle.Render(r.SearchInput.View()) + theme.MutedStyle.Render(searchInfo)
		visibleContent = searchBar + "\n" + visibleContent
	}

	return theme.FocusedBorderStyle.Width(r.Width).Height(r.Height).
		Render(visibleContent)
}
