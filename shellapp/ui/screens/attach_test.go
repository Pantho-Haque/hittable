package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/hittable/shellapp/ui/components/requesteditor"
	zone "github.com/lrstanley/bubblezone"
)

func openBody(t *testing.T, m *MainScreen, hit string) {
	t.Helper()
	m.openFileRaw(hit)
	m.focusBody()
}

func headers(t *testing.T, m *MainScreen) map[string]string {
	t.Helper()
	h := m.Headers.GetContent()
	if h == nil {
		t.Fatalf("headers are not valid JSON: %s", m.Headers.TextArea.Value())
	}
	return h
}

// alt+t (and ⌥t's †) cycles json → text → form → multipart → file → json,
// setting the Content-Type this app owns and seeding a body the type needs.
func TestBodyTypeCycle(t *testing.T) {
	m, hd := newTestScreen(t)
	openBody(t, m, filepath.Join(hd, "a.hit"))
	if m.bodyType() != requesteditor.BodyJSON {
		t.Fatalf("an empty body starts as json, got %v", m.bodyType())
	}
	steps := []struct {
		key  string
		typ  requesteditor.BodyType
		ct   string
		body string
	}{
		{"t", requesteditor.BodyText, "text/plain", ""},
		{"t", requesteditor.BodyForm, "application/x-www-form-urlencoded", ""},
		{"†", requesteditor.BodyMultipart, "multipart/form-data", "{}"},
		{"t", requesteditor.BodyFile, "", "@"},
		{"t", requesteditor.BodyJSON, "application/json", "@"},
	}
	for _, s := range steps {
		if s.key == "†" {
			runes(m, s.key)
		} else {
			alt(m, s.key)
		}
		h := headers(t, m)
		if m.bodyType() != s.typ || h["Content-Type"] != s.ct || m.Body.GetContent() != s.body {
			t.Fatalf("→ %v: type %v, headers %v, body %q", s.typ, m.bodyType(), h, m.Body.GetContent())
		}
		if !strings.Contains(ansi.Strip(m.View()), "type:") {
			t.Fatal("the type line is not drawn")
		}
	}
	// Typing a path after the seeded @ makes it a file body.
	m.setBodyType(requesteditor.BodyFile)
	m.Body.SetValue("@files/a.png")
	if m.bodyType() != requesteditor.BodyFile {
		t.Errorf("@path is a file body, got %v", m.bodyType())
	}
}

// A Content-Type the user typed is theirs: switching type leaves it alone.
func TestBodyTypeKeepsCustomContentType(t *testing.T) {
	m, hd := newTestScreen(t)
	hit := filepath.Join(hd, "c.hit")
	os.WriteFile(hit, []byte(`{"method":"POST","url":"x.test","headers":{"content-type":"application/vnd.api+json","X":"1"},"params":{},"body":"{\"a\":1}","response":null}`), 0o644)
	openBody(t, m, hit)
	alt(m, "t")
	if h := headers(t, m); h["content-type"] != "application/vnd.api+json" || len(h) != 2 {
		t.Errorf("custom content type changed: %v", h)
	}
	// One this app set is replaced in place, keeping the user's key casing.
	os.WriteFile(hit, []byte(`{"method":"POST","url":"x.test","headers":{"content-type":"application/json"},"params":{},"body":"{\"a\":1}","response":null}`), 0o644)
	m2, _ := newTestScreen(t)
	openBody(t, m2, hit)
	m2.setBodyType(requesteditor.BodyMultipart)
	if h := headers(t, m2); h["content-type"] != "multipart/form-data" || len(h) != 1 {
		t.Errorf("auto content type not replaced: %v", h)
	}
	if m2.Body.GetContent() != `{"a":1}` {
		t.Errorf("a JSON object body is kept for multipart: %q", m2.Body.GetContent())
	}
}

// alt+a opens the palette in attach mode; picking a file writes it into the
// multipart body relative to the .hit file, and the palette opens files again
// afterwards — also after a cancelled attach.
func TestAttachFromPalette(t *testing.T) {
	m, hd := newTestScreen(t)
	os.MkdirAll(filepath.Join(hd, "api", "files"), 0o755)
	os.WriteFile(filepath.Join(hd, "api", "files", "avatar.png"), make([]byte, 2048), 0o644)
	hit := filepath.Join(hd, "api", "up.hit")
	os.WriteFile(hit, []byte(`{"method":"POST","url":"x.test","headers":{"Content-Type":"multipart/form-data"},"params":{},"body":"{\n  \"name\": \"x\"\n}","response":null}`), 0o644)
	openBody(t, m, hit)

	alt(m, "a")
	if !m.Palette.Open || !m.attaching {
		t.Fatal("alt+a opens the palette to attach")
	}
	typeText(m, "avatar")
	ctrl(m, tea.KeyEnter)
	want := "{\n  \"name\": \"x\",\n  \"file\": \"@files/avatar.png\"\n}"
	if m.Body.GetContent() != want || m.ActiveFile != hit || m.Focus != FocusBody {
		t.Fatalf("body %q active %s focus %v", m.Body.GetContent(), m.ActiveFile, m.Focus)
	}
	if v := ansi.Strip(m.View()); !strings.Contains(v, "✓ files/avatar.png 2.0 KB") {
		t.Errorf("file list missing:\n%s", v)
	}

	// Cancel, then the palette opens files again.
	alt(m, "a")
	ctrl(m, tea.KeyEsc)
	if m.attaching || m.Palette.Open || m.Focus != FocusBody {
		t.Fatalf("esc cancels the attach: attaching=%v open=%v", m.attaching, m.Palette.Open)
	}
	ctrl(m, tea.KeyCtrlP)
	typeText(m, "notes")
	ctrl(m, tea.KeyEnter)
	if m.ActiveFile != filepath.Join(hd, "notes.md") {
		t.Errorf("the palette still attaches after a cancel: active=%s", m.ActiveFile)
	}
}

// Single-file mode has no palette: alt+a opens the path prompt, Tab
// completes, and a file outside the .hit's folder is stored absolute. A
// missing file shows ✗.
func TestAttachPathPrompt(t *testing.T) {
	tmp := t.TempDir()
	hit := filepath.Join(tmp, "api", "a.hit")
	os.MkdirAll(filepath.Join(tmp, "api", "files"), 0o755)
	os.WriteFile(filepath.Join(tmp, "api", "files", "report.pdf"), []byte("pdf"), 0o644)
	os.WriteFile(hit, []byte(`{"method":"PUT","url":"x.test","headers":{},"params":{},"body":"","response":null}`), 0o644)
	outside := filepath.Join(t.TempDir(), "elsewhere.bin")
	os.WriteFile(outside, []byte("x"), 0o644)

	m := NewMainScreen(filepath.Dir(hit), zone.New())
	m.SetSize(120, 30)
	m.EnterSingleFile(hit)
	m.focusBody()

	alt(m, "a")
	if !m.PathPrompt.Open || m.Palette.Open {
		t.Fatal("alt+a opens the path prompt in single-file mode")
	}
	typeText(m, "fi")
	ctrl(m, tea.KeyTab)
	ctrl(m, tea.KeyTab)
	if m.PathPrompt.Value != "files/report.pdf" {
		t.Fatalf("tab completion: %q", m.PathPrompt.Value)
	}
	ctrl(m, tea.KeyEnter)
	if m.Body.GetContent() != "@files/report.pdf" || m.bodyType() != requesteditor.BodyFile {
		t.Fatalf("binary body: %q %v", m.Body.GetContent(), m.bodyType())
	}

	alt(m, "a")
	typeText(m, outside)
	ctrl(m, tea.KeyEnter)
	if m.Body.GetContent() != "@"+filepath.ToSlash(outside) {
		t.Errorf("outside file: %q", m.Body.GetContent())
	}
	os.Remove(outside)
	if v := ansi.Strip(m.View()); !strings.Contains(v, "✗ "+filepath.ToSlash(outside)+" missing") {
		t.Errorf("missing file not flagged:\n%s", v)
	}

	// A JSON body is not overwritten by an attach.
	m.Body.SetValue(`{"a":1}`)
	m.Body.Choose(requesteditor.BodyJSON)
	alt(m, "a")
	if m.PathPrompt.Open || !strings.Contains(m.StatusBar, "multipart or file") {
		t.Errorf("attach on a JSON body: prompt=%v status=%q", m.PathPrompt.Open, m.StatusBar)
	}
}

func TestAttachToMultipart(t *testing.T) {
	cases := []struct{ body, at, want string }{
		// cursor inside a value: that value becomes the file
		{`{"a": "x", "f": "old"}`, `ol`, `{"a": "x", "f": "@p.png"}`},
		// cursor outside any value: a new member
		{"{\n  \"a\": \"x\"\n}", `{`, "{\n  \"a\": \"x\",\n  \"file\": \"@p.png\"\n}"},
		// cursor in a key: a new member, with the next free name
		{`{"file": "1"}`, `fi`, "{\"file\": \"1\",\n  \"file2\": \"@p.png\"\n}"},
		{`{}`, `{`, "{\n  \"file\": \"@p.png\"\n}"},
		{``, ``, "{\n  \"file\": \"@p.png\"\n}"},
	}
	for _, c := range cases {
		off := strings.Index(c.body, c.at) + len(c.at)
		if got := requesteditor.AttachToMultipart(c.body, off, "p.png"); got != c.want {
			t.Errorf("%q at %q:\n got %q\nwant %q", c.body, c.at, got, c.want)
		}
	}
}

// The type names are buttons, and a click in the editor still lands on the
// text under the mouse with the type line above it.
func TestBodyTypeClickAndEditorClick(t *testing.T) {
	m, hd := newTestScreen(t)
	z := m.Zones
	openBody(t, m, filepath.Join(hd, "a.hit"))
	m.Body.SetValue("line0\nline1\nline2")
	scan(m, z)
	b := z.Get("btype_multipart")
	if b == nil || b.IsZero() {
		t.Fatal("no zone for the multipart button")
	}
	m.handleMouse(press(b.StartX+1, b.StartY))
	m.handleMouse(release(b.StartX+1, b.StartY))
	if m.bodyType() != requesteditor.BodyMultipart || headers(t, m)["Content-Type"] != "multipart/form-data" {
		t.Fatalf("click: type %v headers %v", m.bodyType(), headers(t, m))
	}

	m.Body.SetValue("line0\nline1\nline2")
	scan(m, z)
	ez := z.Get("editor")
	m.handleMouse(press(ez.StartX+1+m.Body.GutterWidth()+2, ez.StartY+1+1)) // inside the border, second row
	m.handleMouse(release(ez.StartX+1+m.Body.GutterWidth()+2, ez.StartY+1+1))
	if row, col := m.Body.Cursor(); row != 1 || col != 2 {
		t.Errorf("editor click landed at %d:%d, want 1:2", row, col)
	}
}
