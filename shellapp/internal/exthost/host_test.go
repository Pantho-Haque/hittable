package exthost

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hittable/shellapp/internal/extensions"
	"github.com/hittable/shellapp/internal/hithome"
)

// recorder collects what the host sends back, and answers requests the way
// a user would.
type recorder struct {
	mu     sync.Mutex
	notes  map[string][]json.RawMessage
	asked  []string
	answer func(r *Request)
}

func (r *recorder) notify(method string, params json.RawMessage) {
	r.mu.Lock()
	r.notes[method] = append(r.notes[method], params)
	r.mu.Unlock()
}

func (r *recorder) waitFor(t *testing.T, method string, ok func(json.RawMessage) bool) json.RawMessage {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		for _, p := range r.notes[method] {
			if ok == nil || ok(p) {
				r.mu.Unlock()
				return p
			}
		}
		r.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t.Fatalf("no %s notification; got %v", method, keys(r.notes))
	return nil
}

func keys(m map[string][]json.RawMessage) []string {
	var out []string
	for k, v := range m {
		out = append(out, k+":"+string(v[len(v)-1]))
	}
	return out
}

func nodeOrSkip(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("HITTABLE_NODE"); p != "" {
		return p
	}
	p, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	return p
}

// installFixture copies testdata/sample into the extensions folder and
// returns its index entry.
func installFixture(t *testing.T) extensions.Installed {
	t.Helper()
	dir := filepath.Join(hithome.Extensions(), "hittable-test.sample-1.0.0")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"package.json", "extension.js"} {
		b, err := os.ReadFile(filepath.Join("testdata", "sample", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, nls, err := extensions.ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	return extensions.Installed{ID: m.ID(), Version: m.Version, Dir: filepath.Base(dir), Enabled: true, Manifest: m, NLS: nls}
}

func TestHostRunsAnExtension(t *testing.T) {
	node := nodeOrSkip(t)
	t.Setenv("HITTABLE_HOME", t.TempDir())
	ext := installFixture(t)
	root := t.TempDir()
	file := filepath.Join(root, "notes.txt")

	rec := &recorder{notes: map[string][]json.RawMessage{}}
	rec.answer = func(r *Request) {
		rec.mu.Lock()
		rec.asked = append(rec.asked, r.Method+" "+string(r.Params))
		rec.mu.Unlock()
		switch r.Method {
		case "window/showMessage":
			r.Reply(0, nil) // the first button, "OK"
		case "languages/idFor":
			r.Reply("plaintext", nil)
		default:
			r.Reply(nil, nil)
		}
	}
	exited := make(chan struct{})
	h, err := Start(context.Background(), Options{
		Node: node, Root: root, Extensions: []extensions.Installed{ext},
		Settings:  Settings{User: map[string]any{"sample.greeting": "Hi"}},
		OnRequest: func(r *Request) { go rec.answer(r) },
		OnNotify:  rec.notify,
		OnExit:    func(error, string) { close(exited) },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		h.Close()
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			t.Error("host did not exit after Close")
		}
	}()

	ctx := context.Background()
	text := "first line   \nleft a TODO here\n"
	if err := h.Call(ctx, "doc/open", map[string]any{"path": file, "languageId": "plaintext", "version": 1, "text": text}, nil); err != nil {
		t.Fatal(err)
	}
	rec.waitFor(t, "ext/activated", nil)

	// Diagnostics arrive batched, with UTF-16 ranges.
	d := rec.waitFor(t, "diagnostics/set", func(p json.RawMessage) bool { return strings.Contains(string(p), "TODO") })
	if !strings.Contains(string(d), `"start":{"line":1,"character":7}`) {
		t.Errorf("diagnostic range: %s", d)
	}
	rec.waitFor(t, "statusBar/set", func(p json.RawMessage) bool { return strings.Contains(string(p), `"text":"sample ready"`) })

	// A command that asks the user, with the setting from user settings.
	var pick string
	if err := h.Call(ctx, "command/execute", map[string]any{"id": "sample.hello", "args": []any{}}, &pick); err != nil {
		t.Fatal(err)
	}
	if pick != "OK" {
		t.Errorf("command returned %q, want OK", pick)
	}
	rec.mu.Lock()
	asked := strings.Join(rec.asked, "\n")
	rec.mu.Unlock()
	if !strings.Contains(asked, `"message":"Hi from sample"`) {
		t.Errorf("message box: %s", asked)
	}

	var comp struct {
		ResultID int `json:"resultId"`
		Items    []struct {
			Label, InsertText string
			Snippet           bool
		} `json:"items"`
	}
	if err := h.Call(ctx, "provide/completion", map[string]any{"path": file, "position": map[string]int{"line": 0, "character": 2}}, &comp); err != nil {
		t.Fatal(err)
	}
	if len(comp.Items) != 2 || comp.Items[1].InsertText != "betaFunc(${1:x})" || !comp.Items[1].Snippet {
		t.Errorf("completion: %+v", comp.Items)
	}

	var hover struct{ Markdown string }
	if err := h.Call(ctx, "provide/hover", map[string]any{"path": file, "position": map[string]int{"line": 1, "character": 1}}, &hover); err != nil {
		t.Fatal(err)
	}
	if hover.Markdown != "**left**" {
		t.Errorf("hover: %q", hover.Markdown)
	}

	var edits []struct {
		Range   struct{ Start, End struct{ Line, Character int } }
		NewText string
	}
	if err := h.Call(ctx, "provide/formatting", map[string]any{"path": file}, &edits); err != nil {
		t.Fatal(err)
	}
	if len(edits) != 1 || edits[0].Range.Start.Character != 10 || edits[0].Range.End.Character != 13 {
		t.Errorf("formatting: %+v", edits)
	}

	// Edits flow back as document changes; fixing the TODO clears it.
	if err := h.Call(ctx, "doc/change", map[string]any{"path": file, "version": 2, "text": "first line\nall done\n"}, nil); err != nil {
		t.Fatal(err)
	}
	rec.waitFor(t, "diagnostics/set", func(p json.RawMessage) bool { return strings.Contains(string(p), `"diagnostics":[]`) })

	// console.log from the extension is captured, never written to the pipe.
	rec.waitFor(t, "log", func(p json.RawMessage) bool { return strings.Contains(string(p), "sample activating") })
}

func TestUTF16Columns(t *testing.T) {
	line := "a😀b"
	if got := ToUTF16(line, 2); got != 3 {
		t.Errorf("ToUTF16 = %d, want 3", got)
	}
	if got := FromUTF16(line, 3); got != 2 {
		t.Errorf("FromUTF16 = %d, want 2", got)
	}
	if got := FromUTF16(line, 99); got != 3 {
		t.Errorf("FromUTF16 past end = %d", got)
	}
}

func TestWidenCSP(t *testing.T) {
	got := widenCSP("default-src 'none'; script-src 'nonce-abc'; style-src http://x", "'sha256-H'", "http://127.0.0.1:9")
	for _, want := range []string{"script-src 'nonce-abc' 'sha256-H'", "connect-src http://127.0.0.1:9", "default-src 'none'"} {
		if !strings.Contains(got, want) {
			t.Errorf("widenCSP = %q, missing %q", got, want)
		}
	}
}

func TestWebviewPanelReachesTheServer(t *testing.T) {
	node := nodeOrSkip(t)
	t.Setenv("HITTABLE_HOME", t.TempDir())
	ext := installFixture(t)
	web, err := StartWebviewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer web.Close()
	rec := &recorder{notes: map[string][]json.RawMessage{}}
	h, err := Start(context.Background(), Options{Node: node, Root: t.TempDir(), Extensions: []extensions.Installed{ext},
		WebviewBase: web.Base(),
		OnRequest:   func(r *Request) { go r.Reply(nil, nil) },
		OnNotify:    rec.notify})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.Call(context.Background(), "command/execute", map[string]any{"id": "sample.panel", "args": []any{}}, nil); err != nil {
		t.Fatal(err)
	}
	var created struct {
		Handle string
		Roots  []string
	}
	_ = json.Unmarshal(rec.waitFor(t, "webview/create", nil), &created)
	var html struct{ HTML string }
	_ = json.Unmarshal(rec.waitFor(t, "webview/html", nil), &html)
	if !strings.Contains(html.HTML, web.Base()+"/res/"+created.Handle+"/") {
		t.Errorf("asWebviewUri did not point at the server: %s", html.HTML)
	}

	// The page, served with the bridge and a widened CSP.
	web.Create(created.Handle, "Sample Panel", created.Roots)
	web.SetHTML(created.Handle, html.HTML)
	page := get(t, web.URL(created.Handle), web.host)
	for _, want := range []string{"acquireVsCodeApi", "connect-src http://" + web.host, "'sha256-"} {
		if !strings.Contains(page, want) {
			t.Errorf("page is missing %q", want)
		}
	}
	// A resource inside the extension is served; one outside is refused.
	src := html.HTML[strings.Index(html.HTML, `src="`)+5:]
	src = src[:strings.Index(src, `"`)]
	if body := get(t, src, web.host); !strings.Contains(body, `"sample"`) {
		t.Errorf("resource not served: %q", body)
	}
	if code := status(t, web.Base()+"/res/"+created.Handle+"/etc/hosts", web.host); code != 403 {
		t.Errorf("a file outside the roots got %d, want 403", code)
	}
	if code := status(t, web.URL(created.Handle), "evil.example:80"); code != 403 {
		t.Errorf("a foreign Host header got %d, want 403", code)
	}
	if code := status(t, "http://"+web.host+"/wrong-token/webview/"+created.Handle, web.host); code != 404 {
		t.Errorf("a wrong token got %d, want 404", code)
	}

	// A message from the page reaches the extension, which echoes it back.
	web.OnMessage = func(handle string, msg json.RawMessage) {
		_ = h.Notify("webview/message", map[string]any{"handle": handle, "message": msg})
	}
	resp, err := http.Post(web.Base()+"/post/"+created.Handle, "application/json", strings.NewReader(`{"ping":1}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	rec.waitFor(t, "webview/post", func(p json.RawMessage) bool { return strings.Contains(string(p), `"echo":{"ping":1}`) })
}

func get(t *testing.T, u, host string) string {
	t.Helper()
	req, _ := http.NewRequest("GET", u, nil)
	req.Host = host
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func status(t *testing.T, u, host string) int {
	t.Helper()
	req, _ := http.NewRequest("GET", u, nil)
	req.Host = host
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}
