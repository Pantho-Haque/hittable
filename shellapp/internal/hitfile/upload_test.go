package hitfile

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Multipart: every part arrives with its name, filename, content type and
// exact bytes, in body order, and the user's bare multipart header is
// replaced by one carrying the boundary.
func TestSendMultipart(t *testing.T) {
	dir := t.TempDir()
	avatar := make([]byte, 3<<20+7) // over the 1MB pipe chunks, odd size
	for i := range avatar {
		avatar[i] = byte(i * 31)
	}
	writeFile(t, filepath.Join(dir, "files", "avatar.png"), avatar)
	writeFile(t, filepath.Join(dir, "files", "notes.bin"), []byte("hello"))

	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header.Values("Content-Type")) != 1 || !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data; boundary=") {
			t.Errorf("content-type: %q", r.Header.Values("Content-Type"))
		}
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(p)
			seen = append(seen, p.FormName()+"|"+p.FileName()+"|"+p.Header.Get("Content-Type")+"|"+sum(b))
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	h := &HitFile{
		Method:  "POST",
		URL:     srv.URL,
		Headers: map[string]string{"content-type": "multipart/form-data", "X-A": "1"},
		Body:    `{"name":"Tanvir","avatar":"@files/avatar.png","n":42,"doc":"@files/notes.bin;type=text/x-note;filename=n.txt","handle":"@@me"}`,
	}
	resp, err := ExecuteAndCaptureContext(t.Context(), h, nil, dir)
	if err != nil || resp.Status != 200 {
		t.Fatalf("send: %v %+v", err, resp)
	}
	want := []string{
		"name|||" + sum([]byte("Tanvir")),
		"avatar|avatar.png|image/png|" + sum(avatar),
		"n|||" + sum([]byte("42")),
		"doc|n.txt|text/x-note|" + sum([]byte("hello")),
		"handle|||" + sum([]byte("@me")),
	}
	if strings.Join(seen, "\n") != strings.Join(want, "\n") {
		t.Errorf("parts:\n%s\nwant\n%s", strings.Join(seen, "\n"), strings.Join(want, "\n"))
	}
}

func sum(b []byte) string {
	return fmt.Sprintf("%d:%x", len(b), sha256.Sum256(b))
}

// Binary: the file's bytes are the body, with a Content-Length, and a <<VAR>>
// in the path resolves first. The type comes from the file when the request
// sets none.
func TestSendBinaryWithVariablePath(t *testing.T) {
	dir := t.TempDir()
	data := []byte("%PDF-1.7 not really")
	writeFile(t, filepath.Join(dir, "out", "report.pdf"), data)
	var gotLen int64
	var gotCT string
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLen, gotCT = r.ContentLength, r.Header.Get("Content-Type")
		got, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()

	h := &HitFile{Method: "PUT", URL: srv.URL, Body: "  @<<DIR>>/report.pdf\n"}
	if _, err := ExecuteAndCaptureContext(t.Context(), h, map[string]string{"DIR": "out"}, dir); err != nil {
		t.Fatal(err)
	}
	if gotLen != int64(len(data)) || string(got) != string(data) || gotCT != "application/pdf" {
		t.Errorf("len %d ct %q body %q", gotLen, gotCT, got)
	}
	// The request's own Content-Type wins; an absolute path works too.
	h = &HitFile{Method: "PUT", URL: srv.URL, Headers: map[string]string{"Content-Type": "application/x-custom"},
		Body: "@" + filepath.Join(dir, "out", "report.pdf")}
	if _, err := ExecuteAndCaptureContext(t.Context(), h, nil, t.TempDir()); err != nil || gotCT != "application/x-custom" {
		t.Errorf("custom type: %v %q", err, gotCT)
	}
}

// A missing file stops the send before anything reaches the server, and the
// error names the path. One good file next to a missing one sends nothing.
func TestSendMissingFileSendsNothing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ok.txt"), []byte("x"))
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()
	for _, h := range []*HitFile{
		{Method: "PUT", URL: srv.URL, Body: "@files/nope.png"},
		{Method: "POST", URL: srv.URL, Headers: map[string]string{"Content-Type": "multipart/form-data"},
			Body: `{"a":"@ok.txt","b":"@files/nope.png"}`},
		{Method: "POST", URL: srv.URL, Body: "@."}, // a folder
	} {
		_, err := ExecuteAndCaptureContext(t.Context(), h, nil, dir)
		if err == nil || !strings.Contains(err.Error(), strings.Trim(h.Body[strings.LastIndex(h.Body, "@")+1:], `"}`)) {
			t.Errorf("%s: err %v should name the path", h.Body, err)
		}
	}
	if _, err := ExecuteAndCaptureContext(t.Context(), &HitFile{Method: "POST", URL: srv.URL,
		Headers: map[string]string{"Content-Type": "multipart/form-data"}, Body: "not json"}, nil, dir); err == nil {
		t.Error("a malformed multipart body is an error")
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("server got %d requests", n)
	}
}

// `@@x` sends `@x`; a multi-line body starting with @ goes as written.
func TestSendRawEscape(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, string(b))
	}))
	defer srv.Close()
	for _, body := range []string{"@@handle", "@one\n@two"} {
		if _, err := ExecuteAndCapture(&HitFile{Method: "POST", URL: srv.URL, Body: body}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if got[0] != "@handle" || got[1] != "@one\n@two" {
		t.Errorf("got %q", got)
	}
}

func TestAsCurlFiles(t *testing.T) {
	cases := []struct {
		r    ResolvedRequest
		want string
	}{
		{ResolvedRequest{Method: "POST", URL: "http://a.test",
			Headers: map[string]string{"Content-Type": "multipart/form-data", "X": "1"},
			Body:    `{"name":"O'Brien","f":"@files/a b.png;type=image/png;filename=me.png","h":"@@x","n":1}`},
			`curl -X POST -H 'X: 1' -F 'name=O'\''Brien' -F 'f=@files/a b.png;type=image/png;filename=me.png' --form-string 'h=@x' -F 'n=1' 'http://a.test'`},
		{ResolvedRequest{Method: "PUT", URL: "http://a.test", Body: "@files/r.pdf"},
			`curl -X PUT -H 'Content-Type: application/pdf' --data-binary '@files/r.pdf' 'http://a.test'`},
		{ResolvedRequest{Method: "PUT", URL: "http://a.test", Headers: map[string]string{"content-type": "x/y"}, Body: "@r.pdf"},
			`curl -X PUT -H 'Content-Type: x/y' --data-binary '@r.pdf' 'http://a.test'`},
		{ResolvedRequest{Method: "POST", URL: "http://a.test", Body: "@@me"},
			`curl -X POST --data-raw '@me' 'http://a.test'`},
		{ResolvedRequest{Method: "POST", URL: "http://a.test", Body: "@a\n@b"},
			`curl -X POST --data-raw '@a
@b' 'http://a.test'`},
	}
	for _, c := range cases {
		if got := c.r.AsCurl(); got != c.want {
			t.Errorf("AsCurl =\n%s\nwant\n%s", got, c.want)
		}
	}
}

// An upload is not bound by the whole-request timeout (a big file takes as
// long as it takes); a plain request still is.
func TestUploadOutlivesRequestTimeout(t *testing.T) {
	old := requestTimeout
	requestTimeout = 100 * time.Millisecond
	defer func() { requestTimeout = old }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond) // a slow link, as the server sees it
		io.Copy(io.Discard, r.Body)
	}))
	defer srv.Close()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.bin"), []byte("data"))
	if _, err := ExecuteAndCaptureContext(t.Context(), &HitFile{Method: "PUT", URL: srv.URL, Body: "@a.bin"}, nil, dir); err != nil {
		t.Errorf("upload cut off: %v", err)
	}
	if _, err := ExecuteAndCaptureContext(t.Context(), &HitFile{Method: "PUT", URL: srv.URL, Body: "text"}, nil, dir); err == nil {
		t.Error("a plain request should still time out")
	}
}
