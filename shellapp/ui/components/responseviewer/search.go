package responseviewer

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (r *ResponseViewer) OpenSearch() {
	r.SearchOpen = true
	r.ShowHeaders = false
	r.SearchInput.SetValue("")
	r.SearchInput.Focus()
	r.SearchQuery = ""
	r.SearchMatches = nil
	r.SearchIdx = -1
}

func (r *ResponseViewer) CloseSearch() {
	r.SearchOpen = false
	r.SearchInput.Blur()
	r.SearchQuery = ""
	r.SearchMatches = nil
	r.SearchIdx = -1
	r.clampScroll()
}

// doSearch records byte offsets of every case-insensitive match in RawContent.
func (r *ResponseViewer) doSearch() {
	r.SearchMatches = nil
	r.SearchIdx = -1
	if r.SearchQuery == "" {
		return
	}
	query := strings.ToLower(r.SearchQuery)
	content := strings.ToLower(r.RawContent)
	offset := 0
	for {
		idx := strings.Index(content[offset:], query)
		if idx < 0 {
			break
		}
		r.SearchMatches = append(r.SearchMatches, offset+idx)
		offset += idx + 1
	}
	if len(r.SearchMatches) > 0 {
		r.SearchIdx = 0
		r.scrollToMatch()
	}
}

func (r *ResponseViewer) NextMatch() {
	if len(r.SearchMatches) == 0 {
		return
	}
	r.SearchIdx = (r.SearchIdx + 1) % len(r.SearchMatches)
	r.scrollToMatch()
}

func (r *ResponseViewer) PrevMatch() {
	if len(r.SearchMatches) == 0 {
		return
	}
	r.SearchIdx = (r.SearchIdx + len(r.SearchMatches) - 1) % len(r.SearchMatches)
	r.scrollToMatch()
}

// scrollToMatch centres the current match's line in the viewport.
func (r *ResponseViewer) scrollToMatch() {
	if r.SearchIdx < 0 || r.SearchIdx >= len(r.SearchMatches) {
		return
	}
	off := r.SearchMatches[r.SearchIdx]
	if off > len(r.RawContent) {
		off = len(r.RawContent)
	}
	line := strings.Count(r.RawContent[:off], "\n")
	r.ScrollY = line - r.bodyRows()/2
	r.clampScroll()
}

func (r *ResponseViewer) Update(msg tea.Msg) tea.Cmd {
	if r.SearchOpen {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc":
				r.CloseSearch()
				return nil
			case "enter", "ctrl+n", "down":
				r.NextMatch()
				return nil
			case "shift+enter", "ctrl+p", "up":
				r.PrevMatch()
				return nil
			}
		}
		var cmd tea.Cmd
		r.SearchInput, cmd = r.SearchInput.Update(msg)
		if q := r.SearchInput.Value(); q != r.SearchQuery {
			r.SearchQuery = q
			r.doSearch()
		}
		return cmd
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	page := r.bodyRows() - 1
	if page < 1 {
		page = 1
	}
	switch km.String() {
	case "up", "k":
		r.ScrollY--
	case "down", "j":
		r.ScrollY++
	case "pgup", "ctrl+u", "b":
		r.ScrollY -= page
	case "pgdown", "ctrl+d", " ", "f":
		r.ScrollY += page
	case "home", "g":
		r.ScrollY = 0
	case "end", "G":
		r.ScrollY = len(r.lines())
	case "h":
		r.ShowHeaders = !r.ShowHeaders
		r.ScrollY = 0
	case "n":
		r.NextMatch()
	case "N":
		r.PrevMatch()
	}
	r.clampScroll()
	return nil
}

// Scroll moves the viewport (mouse wheel).
func (r *ResponseViewer) Scroll(up bool) {
	if up {
		r.ScrollY -= 3
	} else {
		r.ScrollY += 3
	}
	r.clampScroll()
}

func (r *ResponseViewer) clampScroll() {
	maxScroll := len(r.lines()) - r.bodyRows()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if r.ScrollY > maxScroll {
		r.ScrollY = maxScroll
	}
	if r.ScrollY < 0 {
		r.ScrollY = 0
	}
}
