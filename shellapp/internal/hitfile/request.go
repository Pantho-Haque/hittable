package hitfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"
)

type ResolvedRequest struct {
	Method   string
	URL      string
	Headers  map[string]string
	Params   map[string]string
	Body     string
	RawHTTP  string
}

func Resolve(h *HitFile, env map[string]string) *ResolvedRequest {
	url := interpolateString(h.URL, env)
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
		URL:     url,
		Headers: headers,
		Params:  params,
		Body:    body,
	}
}

func (r *ResolvedRequest) ToHTTPRequest() (*http.Request, error) {
	url := r.URL
	if len(r.Params) > 0 {
		sep := "?"
		for i := 0; i < len(url); i++ {
			if url[i] == '?' {
				sep = "&"
				break
			}
		}
		for k, v := range r.Params {
			url += sep + k + "=" + v
			sep = "&"
		}
	}

	var bodyReader *bytes.Buffer
	if r.Body != "" {
		bodyReader = bytes.NewBufferString(r.Body)
	}

	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequest(r.Method, url, bodyReader)
	} else {
		req, err = http.NewRequest(r.Method, url, nil)
	}
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

	url := r.URL
	if len(r.Params) > 0 {
		sep := "?"
		for i := 0; i < len(url); i++ {
			if url[i] == '?' {
				sep = "&"
				break
			}
		}
		for k, v := range r.Params {
			url += sep + k + "=" + v
			sep = "&"
		}
	}
	parts = append(parts, fmt.Sprintf("'%s'", url))

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
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
	duration := time.Since(start)

	var buf bytes.Buffer
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}
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
