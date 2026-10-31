package hitfile

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
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
	return r.toHTTPRequest(context.Background())
}

func (r *ResolvedRequest) toHTTPRequest(ctx context.Context) (*http.Request, error) {
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, r.FullURL(), body)
	if err != nil {
		return nil, err
	}

	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	return req, nil
}

// AsCurl is a command that runs as-is in a POSIX shell. Headers come out in a
// stable order so the same request always copies the same text.
func (r *ResolvedRequest) AsCurl() string {
	parts := []string{"curl", "-X", r.Method}

	keys := make([]string, 0, len(r.Headers))
	for k := range r.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, "-H", shellQuote(k+": "+r.Headers[k]))
	}

	if r.Body != "" {
		parts = append(parts, "-d", shellQuote(r.Body))
	}

	parts = append(parts, shellQuote(r.FullURL()))
	return strings.Join(parts, " ")
}

// shellQuote single-quotes s; an embedded ' becomes '\” so a value like
// O'Brien cannot end the quoting early.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// UnknownVars lists, once each and in order of appearance, the <<KEY>> names
// in the request that env does not define. They are sent as written, which is
// almost never what was meant, so the UI says so.
func UnknownVars(h *HitFile, env map[string]string) []string {
	texts := []string{h.URL, h.Body}
	for _, m := range []map[string]string{h.Headers, h.Params} {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			texts = append(texts, m[k])
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, s := range texts {
		for {
			i := strings.Index(s, "<<")
			if i < 0 {
				break
			}
			end := strings.Index(s[i+2:], ">>")
			if end < 0 {
				break
			}
			key := s[i+2 : i+2+end]
			if _, ok := env[key]; !ok && !seen[key] {
				seen[key] = true
				out = append(out, key)
			}
			s = s[i+4+end:]
		}
	}
	return out
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
	return ExecuteAndCaptureContext(context.Background(), h, env)
}

// ExecuteAndCaptureContext is ExecuteAndCapture with cancellation: cancelling
// ctx aborts the request and returns ctx.Err().
func ExecuteAndCaptureContext(ctx context.Context, h *HitFile, env map[string]string) (*Response, error) {
	resolved := Resolve(h, env)
	req, err := resolved.toHTTPRequest(ctx)
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

	// A user-set Accept-Encoding turns off Go's transparent gzip, so decode
	// here or the response would be shown as compressed bytes.
	var body io.Reader = resp.Body
	if !resp.Uncompressed && strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		zr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		body = zr
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(body)
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
	var b strings.Builder
	for {
		i := strings.Index(s, "<<")
		if i < 0 {
			break
		}
		end := strings.Index(s[i+2:], ">>")
		if end < 0 {
			break // no closing ">>" anywhere after: the rest is literal
		}
		key := s[i+2 : i+2+end]
		b.WriteString(s[:i])
		if val, ok := env[key]; ok {
			b.WriteString(val)
		} else {
			b.WriteString(s[i : i+4+end])
		}
		s = s[i+4+end:]
	}
	b.WriteString(s)
	return b.String()
}

func DumpRequest(req *http.Request) string {
	data, err := httputil.DumpRequest(req, true)
	if err != nil {
		return ""
	}
	return string(data)
}
