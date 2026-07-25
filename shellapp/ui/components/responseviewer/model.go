// Package responseviewer defines the response viewer model and types.
package responseviewer

import "github.com/charmbracelet/bubbles/textinput"

type ResponseViewer struct {
	Content    string
	RawContent string
	Status     int
	StatusText string
	DurationMs int64
	SizeBytes  int
	Ok         bool
	Width      int
	Height     int
	ScrollY    int

	SearchOpen    bool
	SearchInput   textinput.Model
	SearchQuery   string
	SearchMatches []int
	SearchIdx     int
}

func New() *ResponseViewer {
	ti := textinput.New()
	ti.Placeholder = "search..."
	ti.CharLimit = 200
	ti.Width = 40
	return &ResponseViewer{
		SearchInput: ti,
		SearchIdx:   -1,
	}
}

func (r *ResponseViewer) SetSize(w, h int) {
	r.Width = w
	r.Height = h
}

func (r *ResponseViewer) SetResponse(status int, statusText string, durationMs int64, sizeBytes int, ok bool, body string) {
	r.Status = status
	r.StatusText = statusText
	r.DurationMs = durationMs
	r.SizeBytes = sizeBytes
	r.Ok = ok
	r.Content = body
	r.RawContent = body
	r.ScrollY = 0
	r.SearchMatches = nil
	r.SearchIdx = -1
}
