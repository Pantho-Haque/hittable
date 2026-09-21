// Package responseviewer renders the HTTP response panel: status line,
// Chroma-highlighted body (cached per response), a headers view, and search.
package responseviewer

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
)

// maxHighlightBytes caps Chroma highlighting; larger bodies render plain.
const maxHighlightBytes = 256 * 1024

type ResponseViewer struct {
	Content    string // pretty body shown to the user
	RawContent string // same as Content; kept for search coordinate mapping
	Status     int
	StatusText string
	DurationMs int64
	SizeBytes  int
	Ok         bool
	Headers    map[string]string
	Width      int
	Height     int
	ScrollY    int

	ShowHeaders bool

	SearchOpen    bool
	SearchInput   textinput.Model
	SearchQuery   string
	SearchMatches []int
	SearchIdx     int

	// Render cache: highlighted body lines, computed once per SetResponse.
	bodyLines   []string // highlighted
	rawLines    []string // plain, for search
	headerLines []string
}

func New() *ResponseViewer {
	ti := textinput.New()
	ti.Placeholder = "search…"
	ti.Prompt = "/ "
	ti.CharLimit = 200
	ti.Width = 40
	return &ResponseViewer{SearchInput: ti, SearchIdx: -1}
}

func (r *ResponseViewer) SetSize(w, h int) {
	r.Width = w
	r.Height = h
	r.clampScroll()
}

// Empty reports whether no response has been set.
func (r *ResponseViewer) Empty() bool { return r.Content == "" && r.Status == 0 }

// SetResponse replaces the panel content. body is pretty-printed if it is
// JSON; otherwise shown verbatim.
func (r *ResponseViewer) SetResponse(status int, statusText string, durationMs int64, sizeBytes int, ok bool, body string) {
	r.Status = status
	r.StatusText = statusText
	r.DurationMs = durationMs
	r.SizeBytes = sizeBytes
	r.Ok = ok
	r.ScrollY = 0
	r.SearchMatches = nil
	r.SearchIdx = -1
	r.ShowHeaders = false

	var pretty bytes.Buffer
	if body != "" && json.Indent(&pretty, []byte(body), "", "  ") == nil {
		body = pretty.String()
		if len(body) <= maxHighlightBytes {
			r.bodyLines = strings.Split(r.highlightJSON(body), "\n")
		} else {
			r.bodyLines = strings.Split(body, "\n") // ponytail: plain text above the cap keeps SetResponse instant
		}
	} else {
		r.bodyLines = strings.Split(body, "\n")
	}
	r.Content = body
	r.RawContent = body
	r.rawLines = strings.Split(body, "\n")
	if r.Status == 0 && body == "" {
		r.bodyLines = nil
		r.rawLines = nil
	}
	r.SetHeaders(nil)
	if r.SearchQuery != "" {
		r.doSearch()
	}
}

// SetHeaders stores response headers for the headers view (h key).
func (r *ResponseViewer) SetHeaders(h map[string]string) {
	r.Headers = h
	r.headerLines = r.headerLines[:0]
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r.headerLines = append(r.headerLines, k+": "+h[k])
	}
	if len(r.headerLines) == 0 {
		r.headerLines = []string{"(no headers)"}
	}
}

// BodyForClipboard returns the plain body text.
func (r *ResponseViewer) BodyForClipboard() string { return r.Content }
