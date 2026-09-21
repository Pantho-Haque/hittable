package hitfile

import (
	"bytes"
	"encoding/json"
)

type HitFile struct {
	Method   string            `json:"method"`
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers"`
	Params   map[string]string `json:"params"`
	Body     string            `json:"body"`
	Response *Response         `json:"response"`
}

type Response struct {
	Data       interface{}       `json:"data"`
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Ok         bool              `json:"ok"`
	Headers    map[string]string `json:"headers"`
	Cookies    map[string]string `json:"cookies"`
	DurationMs int64             `json:"durationMs"`
	SizeBytes  int               `json:"sizeBytes"`
}

func Parse(data []byte) (*HitFile, error) {
	var h HitFile
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, err
	}
	if h.Headers == nil {
		h.Headers = map[string]string{}
	}
	if h.Params == nil {
		h.Params = map[string]string{}
	}
	return &h, nil
}

// Marshal renders h as indented JSON without HTML escaping, so <<KEY>>
// templates stay readable and byte-compatible with the web app's files.
func Marshal(h *HitFile) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(h) // Encode appends a trailing newline
	return buf.Bytes()
}
