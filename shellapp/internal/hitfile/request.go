package hitfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type ResolvedRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Params  map[string]string
	Body    string
	RawHTTP string
}

func Resolve(h *HitFile, env map[string]string) *ResolvedRequest {
	u := strings.TrimSpace(interpolateString(h.URL, env))
	if u != "" && !strings.Contains(u, "://") {
		u = "http://" + u
	}
	headers := make(map[string]string, len(h.Headers))
	for k, v := range h.Headers {
		headers[k] = interpolateString(v, env)
	}
	params := make(map[string]string, len(h.Params))
	for k, v := range h.Params {
		params[k] = interpolateString(v, env)
	}
	body := interpolateString(h.Body, env)

	return &ResolvedRequest{
		Method:  h.Method,
		URL:     u,
		Headers: headers,
		Params:  params,
		Body:    body,
	}
}

func (r *ResolvedRequest) ToHTTPRequest() (*http.Request, error) {
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequest(r.Method, r.FullURL(), body)
	if err != nil {
		return nil, err
	}

	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

func (r *ResolvedRequest) AsCurl() string {
	parts := []string{"curl", "-X", r.Method}

	for k, v := range r.Headers {
		parts = append(parts, "-H", fmt.Sprintf("'%s: %s'", k, v))
	}

	if r.Body != "" {
		parts = append(parts, "-d", fmt.Sprintf("'%s'", r.Body))
	}

	parts = append(parts, fmt.Sprintf("'%s'", r.FullURL()))
	return strings.Join(parts, " ")
}

// FullURL appends Params to URL as a URL-encoded query string.
func (r *ResolvedRequest) FullURL() string {
	if len(r.Params) == 0 {
		return r.URL
	}
	q := url.Values{}
	for k, v := range r.Params {
		q.Set(k, v)
	}
	sep := "?"
	if strings.Contains(r.URL, "?") {
		sep = "&"
	}
	return r.URL + sep + q.Encode()
}

func ExecuteAndCapture(h *HitFile, env map[string]string) (*Response, error) {
	resolved := Resolve(h, env)
	req, err := resolved.ToHTTPRequest()
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}
	duration := time.Since(start)
	bodyBytes := buf.Bytes()

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
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		data = string(bodyBytes)
	}

	return &Response{
		Data:       data,
		Status:     resp.StatusCode,
		StatusText: http.StatusText(resp.StatusCode),
		Ok:         resp.StatusCode >= 200 && resp.StatusCode < 300,
		Headers:    respHeaders,
		Cookies:    cookies,
		DurationMs: duration.Milliseconds(),
		SizeBytes:  len(bodyBytes),
	}, nil
}

func interpolateString(s string, env map[string]string) string {
	result := ""
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '<' && s[i+1] == '<' {
			end := i + 2
			for end < len(s) && !(s[end] == '>' && end+1 < len(s) && s[end+1] == '>') {
				end++
			}
			if end < len(s) {
				key := s[i+2 : end]
				if val, ok := env[key]; ok {
					result += val
				} else {
					result += s[i : end+2]
				}
				i = end + 2
				continue
			}
		}
		result += string(s[i])
		i++
	}
	return result
}

func DumpRequest(req *http.Request) string {
	data, err := httputil.DumpRequest(req, true)
	if err != nil {
		return ""
	}
	return string(data)
}
