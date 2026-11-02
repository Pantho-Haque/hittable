package hitfile

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"os"
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
	// BaseDir is the .hit file's folder: relative @file paths in Body are
	// read from there.
	BaseDir string
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
	refs, err := ParseFileRefs(r.Body, ContentType(r.Headers))
	if err != nil {
		return nil, err
	}
	if refs == nil || refs.Kind == "raw" {
		text := r.Body
		if refs != nil {
			text = refs.Body
		}
		var body io.Reader
		if text != "" {
			body = strings.NewReader(text)
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

	// Open every file before anything is sent, so a missing one stops the
	// request with its path named.
	specs := refs.Files()
	files := make([]*os.File, len(specs))
	closeAll := func() {
		for _, f := range files {
			if f != nil {
				f.Close()
			}
		}
	}
	for i, s := range specs {
		f, err := openUpload(r.BaseDir, s.Path)
		if err != nil {
			closeAll()
			return nil, err
		}
		files[i] = f
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, r.FullURL(), nil)
	if err != nil {
		closeAll()
		return nil, err
	}
	for k, v := range r.Headers {
		if !strings.EqualFold(k, "Content-Type") {
			req.Header.Set(k, v)
		}
	}

	if refs.Kind == "binary" {
		fi, _ := files[0].Stat()
		req.Body, req.ContentLength = files[0], fi.Size()
		if req.ContentLength == 0 {
			req.Body = http.NoBody
			files[0].Close()
		}
		req.Header.Set("Content-Type", binaryContentType(r.Headers, refs.File))
		return req, nil
	}

	// Multipart streams through a pipe, so a big file never sits in memory.
	// The transport closes the body on every outcome, which unblocks the writer.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	req.Body, req.ContentLength = pr, -1
	req.Header.Set("Content-Type", mw.FormDataContentType())
	go func() {
		defer closeAll()
		err := writeParts(mw, refs.Parts, files)
		if err == nil {
			err = mw.Close()
		}
		pw.CloseWithError(err)
	}()
	return req, nil
}

// binaryContentType is the request's own Content-Type, else the file's.
func binaryContentType(h map[string]string, f *FileSpec) string {
	if ct := ContentType(h); ct != "" {
		return ct
	}
	return f.MimeType()
}

func openUpload(baseDir, p string) (*os.File, error) {
	full := ResolvePath(baseDir, p)
	f, err := os.Open(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", p)
		}
		return nil, fmt.Errorf("cannot read %s: %w", p, err)
	}
	if fi, err := f.Stat(); err != nil || fi.IsDir() {
		f.Close()
		return nil, fmt.Errorf("not a file: %s", p)
	}
	return f, nil
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"", "\r", "%0D", "\n", "%0A")

func writeParts(mw *multipart.Writer, parts []FilePart, files []*os.File) error {
	fi := 0
	for _, p := range parts {
		if p.File == nil {
			if err := mw.WriteField(p.Name, p.Value); err != nil {
				return err
			}
			continue
		}
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			quoteEscaper.Replace(p.Name), quoteEscaper.Replace(p.File.FileName())))
		h.Set("Content-Type", p.File.MimeType())
		w, err := mw.CreatePart(h)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, files[fi]); err != nil {
			return err
		}
		fi++
	}
	return nil
}

// AsCurl is a command that runs as-is in a POSIX shell. Headers come out in a
// stable order so the same request always copies the same text.
func (r *ResolvedRequest) AsCurl() string {
	parts := []string{"curl", "-X", r.Method}

	refs, err := ParseFileRefs(r.Body, ContentType(r.Headers))
	if err != nil {
		refs = nil // sent as text, which is what the error will say on send
	}
	headers := r.Headers
	multi := refs != nil && refs.Kind == "multipart"
	if multi || refs != nil && refs.Kind == "binary" {
		// curl writes its own multipart Content-Type (with the boundary), and
		// --data-binary would default to form-urlencoded.
		headers = make(map[string]string, len(r.Headers)+1)
		for k, v := range r.Headers {
			if !strings.EqualFold(k, "Content-Type") {
				headers[k] = v
			}
		}
		if !multi {
			headers["Content-Type"] = binaryContentType(r.Headers, refs.File)
		}
	}
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, "-H", shellQuote(k+": "+headers[k]))
	}

	switch {
	case refs == nil && r.Body != "":
		flag := "-d"
		if strings.HasPrefix(r.Body, "@") {
			flag = "--data-raw" // -d @x would read a file
		}
		parts = append(parts, flag, shellQuote(r.Body))
	case refs == nil:
	case refs.Kind == "raw":
		parts = append(parts, "--data-raw", shellQuote(refs.Body))
	case refs.Kind == "binary":
		parts = append(parts, "--data-binary", shellQuote("@"+refs.File.Path))
	default:
		for _, p := range refs.Parts {
			switch {
			case p.File != nil:
				spec := p.Name + "=@" + p.File.Path
				if p.File.Type != "" {
					spec += ";type=" + p.File.Type
				}
				if p.File.Filename != "" {
					spec += ";filename=" + p.File.Filename
				}
				parts = append(parts, "-F", shellQuote(spec))
			case strings.HasPrefix(p.Value, "@") || strings.HasPrefix(p.Value, "<") || strings.Contains(p.Value, ";"):
				// -F would read a file (@, <) or an option (;type=) out of the value.
				parts = append(parts, "--form-string", shellQuote(p.Name+"="+p.Value))
			default:
				parts = append(parts, "-F", shellQuote(p.Name+"="+p.Value))
			}
		}
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
	return ExecuteAndCaptureContext(context.Background(), h, env, "")
}

// requestTimeout bounds a whole request; uploadTransport bounds only the wait
// for response headers once an upload has been written, so a big file is
// never cut off but a dead server still is. Esc (ctx) cancels either.
var (
	requestTimeout  = 30 * time.Second
	uploadTransport = func() *http.Transport {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.ResponseHeaderTimeout = requestTimeout
		return t
	}()
)

// ExecuteAndCaptureContext is ExecuteAndCapture with cancellation: cancelling
// ctx aborts the request and returns ctx.Err(). baseDir is the .hit file's
// folder, which relative @file paths in the body are read from.
func ExecuteAndCaptureContext(ctx context.Context, h *HitFile, env map[string]string, baseDir string) (*Response, error) {
	resolved := Resolve(h, env)
	resolved.BaseDir = baseDir
	req, err := resolved.toHTTPRequest(ctx)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: requestTimeout}
	if req.Body != nil && req.GetBody == nil { // a file upload
		client = &http.Client{Transport: uploadTransport}
	}

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
