// Package responseviewer implements search functionality for the response viewer.
package responseviewer

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (r *ResponseViewer) OpenSearch() {
	r.SearchOpen = true
	r.SearchInput.SetValue("")
	r.SearchInput.Focus()
	r.SearchQuery = ""
	r.SearchMatches = nil
	r.SearchIdx = -1
}

func (r *ResponseViewer) CloseSearch() {
	r.SearchOpen = false
	r.SearchQuery = ""
	r.SearchMatches = nil
	r.SearchIdx = -1
}

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
	}
}

func (r *ResponseViewer) NextMatch() {
	if len(r.SearchMatches) == 0 {
		return
	}
	r.SearchIdx = (r.SearchIdx + 1) % len(r.SearchMatches)
}

func (r *ResponseViewer) PrevMatch() {
	if len(r.SearchMatches) == 0 {
		return
	}
	r.SearchIdx--
	if r.SearchIdx < 0 {
		r.SearchIdx = len(r.SearchMatches) - 1
	}
}

func (r *ResponseViewer) Update(msg tea.Msg) tea.Cmd {
	if r.SearchOpen {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				r.CloseSearch()
				return nil
			case "enter":
				r.NextMatch()
				return nil
			case "shift+enter":
				r.PrevMatch()
				return nil
			}
		}
		var cmd tea.Cmd
		r.SearchInput, cmd = r.SearchInput.Update(msg)
		newQuery := r.SearchInput.Value()
		if newQuery != r.SearchQuery {
			r.SearchQuery = newQuery
			r.doSearch()
		}
		return cmd
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if r.ScrollY > 0 {
				r.ScrollY--
			}
		case "down", "j":
			r.ScrollY++
		case "pgup":
			if r.ScrollY > 0 {
				r.ScrollY -= 5
				if r.ScrollY < 0 {
					r.ScrollY = 0
				}
			}
		case "pgdown":
			r.ScrollY += 5
		}
		r.clampScroll()
	}
	return nil
}

func (r *ResponseViewer) clampScroll() {
	if r.Content == "" {
		r.ScrollY = 0
		return
	}
	maxLines := r.Height - 4
	if r.SearchOpen {
		maxLines -= 2
	}
	if maxLines < 1 {
		maxLines = 1
	}
	contentLines := strings.Split(r.Content, "\n")
	maxScroll := len(contentLines) - maxLines
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
