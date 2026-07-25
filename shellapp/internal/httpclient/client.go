package httpclient

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"time"
)

type RequestResult struct {
	Data       interface{}       `json:"data"`
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Ok         bool              `json:"ok"`
	Headers    map[string]string `json:"headers"`
	Cookies    map[string]string `json:"cookies"`
	DurationMs int64             `json:"durationMs"`
	SizeBytes  int               `json:"sizeBytes"`
	RawBody    string            `json:"-"`
}

func Execute(method, url string, headers map[string]string, params map[string]string, body string) (*RequestResult, error) {
	if len(params) > 0 {
		sep := "?"
		if containsChar(url, '?') {
			sep = "&"
		}
		for k, v := range params {
			url += sep + k + "=" + v
			sep = "&"
		}
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	duration := time.Since(start)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	cookies := make(map[string]string)
	for _, c := range resp.Cookies() {
		cookies[c.Name] = c.Value
	}

	var data interface{}
	if err := json.Unmarshal(respBody, &data); err != nil {
		data = string(respBody)
	}

	statusText := http.StatusText(resp.StatusCode)

	return &RequestResult{
		Data:       data,
		Status:     resp.StatusCode,
		StatusText: statusText,
		Ok:         resp.StatusCode >= 200 && resp.StatusCode < 300,
		Headers:    respHeaders,
		Cookies:    cookies,
		DurationMs: duration.Milliseconds(),
		SizeBytes:  len(respBody),
		RawBody:    string(respBody),
	}, nil
}

func containsChar(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return true
		}
	}
	return false
}

func DumpRequest(req *http.Request) string {
	data, err := httputil.DumpRequest(req, true)
	if err != nil {
		return ""
	}
	return string(data)
}
