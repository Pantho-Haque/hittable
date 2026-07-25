package hitfile

import "encoding/json"

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

func Marshal(h *HitFile) ([]byte, error) {
	return json.MarshalIndent(h, "", "  ")
}

func MarshalWithNewline(h *HitFile) ([]byte, error) {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
