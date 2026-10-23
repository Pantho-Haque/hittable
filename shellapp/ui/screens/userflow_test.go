package screens

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/internal/hitfile"
	"github.com/hittable/shellapp/ui/components/requesteditor"
)

// ---------- helpers ----------

// forwarder is a stand-in tea model: every message the screen's goroutines
// send (responses, debounce ticks, git results) lands on a channel the test
// reads, so the request path runs exactly as in the app.
type forwarder struct{ ch chan tea.Msg }

func (f forwarder) Init() tea.Cmd { return nil }
func (f forwarder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	select {
	case f.ch <- msg:
	default:
	}
	return f, nil
}
func (f forwarder) View() string { return "" }

// withProgram wires a real (headless) tea.Program so sendRequestAsync runs.
func withProgram(t *testing.T) chan tea.Msg {
	t.Helper()
	ch := make(chan tea.Msg, 256)
	p := tea.NewProgram(forwarder{ch}, tea.WithInput(nil), tea.WithOutput(io.Discard),
		tea.WithoutRenderer(), tea.WithoutSignals())
	done := make(chan struct{})
	go func() { _, _ = p.Run(); close(done) }()
	SetTeaProgram(p)
	t.Cleanup(func() {
		SetTeaProgram(nil)
		p.Kill()
		<-done
	})
	return ch
}

// awaitResponse waits for the request goroutine's result and applies it.
func awaitResponse(t *testing.T, m *MainScreen, ch chan tea.Msg) responseMsg {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case msg := <-ch:
			if r, ok := msg.(responseMsg); ok {
				m.Update(r)
				return r
			}
		case <-deadline:
			t.Fatal("no response arrived")
		}
	}
}

func typeText(m *MainScreen, s string) {
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Paste: true})
}

func ctrl(m *MainScreen, k tea.KeyType) (tea.Model, tea.Cmd) { return m.Update(tea.KeyMsg{Type: k}) }

func runes(m *MainScreen, s string) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

// apiProject is a project whose env points at a test server.
func apiProject(t *testing.T, handler http.HandlerFunc) (*MainScreen, string, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	m, hd := newTestScreen(t)
	env := map[string]string{"BASE_URL": srv.URL, "TOKEN": "s3cret"}
	b, _ := json.Marshal(env)
	os.WriteFile(filepath.Join(hd, "env.json"), b, 0o644)
	m.EnvData = env
	return m, hd, srv
}

func writeHit(t *testing.T, path string, h hitfile.HitFile) {
	t.Helper()
	if err := os.WriteFile(path, hitfile.Marshal(&h), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------- Runner: edit, save, reopen ----------

// A user opens a request, rewrites its URL, cycles the method, fills headers
// and a body from the keyboard, saves with ctrl+s, closes and reopens it.
func TestRunnerKeyboardEditSaveReopen(t *testing.T) {
	m, hd := newTestScreen(t)
	hit := filepath.Join(hd, "a.hit")
	m.openFileRaw(hit)
	if m.Focus != FocusURLBar || m.ViewMode != ViewRunner {
		t.Fatalf("a .hit opens in Runner with the URL bar focused, got focus=%d view=%d", m.Focus, m.ViewMode)
	}

	ctrl(m, tea.KeyCtrlU) // clear the URL
	typeText(m, "<<BASE_URL>>/users")
	ctrl(m, tea.KeyCtrlRight) // GET -> POST
	if m.URLBar.Method != "POST" {
		t.Fatalf("ctrl+→ should cycle to POST, got %s", m.URLBar.Method)
	}
	ctrl(m, tea.KeyCtrlLeft)
	ctrl(m, tea.KeyCtrlLeft) // POST -> GET -> OPTIONS (wraps)
	if m.URLBar.Method != "OPTIONS" {
		t.Fatalf("ctrl+← should wrap to OPTIONS, got %s", m.URLBar.Method)
	}
	ctrl(m, tea.KeyCtrlRight)
	ctrl(m, tea.KeyCtrlRight) // -> POST

	ctrl(m, tea.KeyTab) // URL bar -> tab bar
	if m.Focus != FocusTabBar {
		t.Fatalf("tab should move to the tab bar, got %d", m.Focus)
	}
	runes(m, "2")
	if m.ActiveTab != requesteditor.TabHeaders {
		t.Fatalf("2 should pick Headers, got %d", m.ActiveTab)
	}
	ctrl(m, tea.KeyEnter)
	if m.Focus != FocusBody {
		t.Fatalf("enter on the tab bar should edit the tab, got %d", m.Focus)
	}
	ctrl(m, tea.KeyCtrlA)
	typeText(m, `{"Authorization":"Bearer <<TOKEN>>"}`)

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3"), Alt: true}) // alt+3 -> Body
	if m.ActiveTab != requesteditor.TabBody || m.Focus != FocusBody {
		t.Fatalf("alt+3 should jump to Body, got tab %d focus %d", m.ActiveTab, m.Focus)
	}
	ctrl(m, tea.KeyCtrlA)
	typeText(m, `{"name":"ada","n":1}`)
	ctrl(m, tea.KeyCtrlL) // format body JSON
	if m.StatusBar != "Body formatted" || !strings.Contains(m.Body.GetContent(), "\n  \"name\"") {
		t.Fatalf("ctrl+l should pretty-print the body: %q / %q", m.StatusBar, m.Body.GetContent())
	}

	ctrl(m, tea.KeyCtrlS)
	if m.StatusBar != "Saved" {
		t.Errorf("ctrl+s status = %q", m.StatusBar)
	}
	got := readHit(t, hit)
	if got["url"] != "<<BASE_URL>>/users" || got["method"] != "POST" {
		t.Errorf("url/method on disk: %v %v", got["url"], got["method"])
	}
	if h, _ := got["headers"].(map[string]any); h["Authorization"] != "Bearer <<TOKEN>>" {
		t.Errorf("headers on disk: %v", got["headers"])
	}
	var body map[string]any
	if json.Unmarshal([]byte(got["body"].(string)), &body) != nil || body["name"] != "ada" {
		t.Errorf("body on disk: %v", got["body"])
	}
	if raw, _ := os.ReadFile(hit); strings.Contains(string(raw), `\u003c`) {
		t.Errorf("templates were HTML-escaped on disk:\n%s", raw)
	}

	// esc closes (from the URL bar), reopening shows what was saved.
	m.Focus = FocusURLBar
	m.focusCurrent()
	ctrl(m, tea.KeyEsc)
	if m.ActiveFile != "" || !m.ExplorerFocused {
		t.Fatalf("esc should close the file and focus the explorer")
	}
	m.openFileRaw(hit)
	if meth, u := m.URLBar.GetContent(); meth != "POST" || u != "<<BASE_URL>>/users" {
		t.Errorf("reopened url bar: %s %s", meth, u)
	}
	if m.Headers.GetContent()["Authorization"] != "Bearer <<TOKEN>>" {
		t.Errorf("reopened headers: %v", m.Headers.GetContent())
	}
}

// Invalid JSON typed into Params keeps the last good value on disk and flags
// the tab, instead of writing null.
func TestInvalidParamsKeepLastGoodValue(t *testing.T) {
	m, hd := newTestScreen(t)
	hit := filepath.Join(hd, "a.hit")
	m.openFileRaw(hit)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1"), Alt: true})
	ctrl(m, tea.KeyCtrlA)
	typeText(m, `{"page":"2"}`)
	ctrl(m, tea.KeyCtrlS)
	ctrl(m, tea.KeyCtrlA)
	typeText(m, `{"page":`) // half-typed
	ctrl(m, tea.KeyCtrlS)
	if !m.Params.InvalidJSON {
		t.Error("half-typed params should be flagged invalid")
	}
	if p, _ := readHit(t, hit)["params"].(map[string]any); p["page"] != "2" {
		t.Errorf("last valid params lost: %v", readHit(t, hit)["params"])
	}
}

// ctrl+t flips Runner <-> Text over the same content; edits in Text land in
// Runner; invalid JSON in Text blocks the switch back and says why.
func TestRunnerTextToggleRoundTrip(t *testing.T) {
	m, hd := newTestScreen(t)
	hit := filepath.Join(hd, "a.hit")
	m.openFileRaw(hit)
	ctrl(m, tea.KeyCtrlT)
	if m.ViewMode != ViewText || m.Focus != FocusTextEditor {
		t.Fatalf("ctrl+t should open Text view with the editor focused")
	}
	var h hitfile.HitFile
	if err := json.Unmarshal([]byte(m.TextEd.GetContent()), &h); err != nil || h.URL != "<<BASE_URL>>/a" {
		t.Fatalf("text view should be the file's JSON: %v %q", err, m.TextEd.GetContent())
	}
	h.Method, h.Body = "DELETE", "bye"
	m.TextEd.SetContent(hit, string(hitfile.Marshal(&h)))
	ctrl(m, tea.KeyCtrlT)
	if m.ViewMode != ViewRunner || m.URLBar.Method != "DELETE" || m.Body.GetContent() != "bye" {
		t.Fatalf("text edits should reach the runner: %s %q", m.URLBar.Method, m.Body.GetContent())
	}

	ctrl(m, tea.KeyCtrlT)
	m.TextEd.SetContent(hit, `{"method": "GET", "url": `) // broken
	ctrl(m, tea.KeyCtrlT)
	if m.ViewMode != ViewText || !strings.Contains(m.StatusBar, "Invalid JSON") {
		t.Fatalf("invalid JSON must keep Text view and explain: view=%d status=%q", m.ViewMode, m.StatusBar)
	}
	// Fixing it lets the switch through.
	m.TextEd.SetContent(hit, `{"method":"PUT","url":"http://fixed","headers":{},"params":{},"body":"","response":null}`)
	ctrl(m, tea.KeyCtrlT)
	if m.ViewMode != ViewRunner || m.URLBar.Method != "PUT" {
		t.Fatalf("fixed JSON should switch back: view=%d method=%s", m.ViewMode, m.URLBar.Method)
	}
	m.WriteQueue.FlushNow()
	if readHit(t, hit)["url"] != "http://fixed" {
		t.Errorf("toggle should persist: %v", readHit(t, hit)["url"])
	}
}

// Method picker from the keyboard: enter opens, a first letter jumps, enter
// picks; esc cancels without changing.
func TestMethodPickerKeyboard(t *testing.T) {
	m, hd := newTestScreen(t)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	ctrl(m, tea.KeyEnter)
	if m.Focus != FocusMethodDropdown || !m.URLBar.DropdownOpen {
		t.Fatal("enter on the URL bar should open the method picker")
	}
	runes(m, "p") // first P method: POST
	runes(m, "p") // next P method: PUT
	ctrl(m, tea.KeyEnter)
	if m.URLBar.Method != "PUT" || m.URLBar.DropdownOpen || m.Focus != FocusURLBar {
		t.Fatalf("picked %s open=%v focus=%d", m.URLBar.Method, m.URLBar.DropdownOpen, m.Focus)
	}
	ctrl(m, tea.KeyEnter)
	ctrl(m, tea.KeyRight)
	ctrl(m, tea.KeyEsc)
	if m.URLBar.Method != "PUT" || m.URLBar.DropdownOpen {
		t.Fatalf("esc should cancel the picker: %s", m.URLBar.Method)
	}
	if m.ActiveFile == "" {
		t.Fatal("esc in the picker must not close the file")
	}
}

// ---------- Sending requests ----------

// The whole send path through the model: ctrl+r resolves <<KEY>> templates
// from env.json, the server sees them, the response is shown and saved into
// the .hit, and the templates on disk are untouched.
func TestSendRequestEndToEnd(t *testing.T) {
	var mu sync.Mutex
	var seen *http.Request
	var seenBody string
	m, hd, _ := apiProject(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen, seenBody = r.Clone(r.Context()), string(b)
		mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "42"})
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Trace", "t-1")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":7,"name":"ada"}`))
	})
	ch := withProgram(t)
	hit := filepath.Join(hd, "create.hit")
	writeHit(t, hit, hitfile.HitFile{
		Method: "POST", URL: "<<BASE_URL>>/users",
		Headers: map[string]string{"Authorization": "Bearer <<TOKEN>>", "Content-Type": "application/json"},
		Params:  map[string]string{"q": "a b&c"},
		Body:    `{"name":"ada","token":"<<TOKEN>>"}`,
	})
	m.openFileRaw(hit)

	_, cmd := ctrl(m, tea.KeyCtrlR)
	if !m.Sending || cmd == nil {
		t.Fatal("ctrl+r should start sending and tick the spinner")
	}
	if !strings.Contains(m.footerText(), "Sending") {
		t.Errorf("footer while sending = %q", m.footerText())
	}
	// A second ctrl+r while in flight is ignored (no double send).
	if _, cmd2 := ctrl(m, tea.KeyCtrlR); cmd2 != nil {
		t.Error("double send should be guarded")
	}
	awaitResponse(t, m, ch)

	mu.Lock()
	if seen == nil {
		t.Fatal("server never saw the request")
	}
	if seen.Method != "POST" || seen.URL.Path != "/users" || seen.URL.Query().Get("q") != "a b&c" {
		t.Errorf("request line: %s %s", seen.Method, seen.URL)
	}
	if seen.Header.Get("Authorization") != "Bearer s3cret" {
		t.Errorf("header not interpolated: %q", seen.Header.Get("Authorization"))
	}
	if seenBody != `{"name":"ada","token":"s3cret"}` {
		t.Errorf("body not interpolated: %q", seenBody)
	}
	mu.Unlock()

	if m.Sending {
		t.Error("Sending should clear when the response lands")
	}
	if m.Response.Status != 201 || !strings.HasPrefix(m.StatusBar, "201 Created") {
		t.Errorf("response shown: %d %q", m.Response.Status, m.StatusBar)
	}
	if !strings.Contains(m.Response.Content, `"name": "ada"`) {
		t.Errorf("JSON body should be indented in the viewer: %q", m.Response.Content)
	}

	m.WriteQueue.FlushNow()
	disk := readHit(t, hit)
	if disk["url"] != "<<BASE_URL>>/users" || !strings.Contains(disk["body"].(string), "<<TOKEN>>") {
		t.Errorf("send must not bake env values into the file: %v / %v", disk["url"], disk["body"])
	}
	resp, _ := disk["response"].(map[string]any)
	if resp == nil || resp["status"] != float64(201) || resp["ok"] != true || resp["statusText"] != "Created" {
		t.Fatalf("response not persisted: %v", disk["response"])
	}
	if c, _ := resp["cookies"].(map[string]any); c["sid"] != "42" {
		t.Errorf("cookies: %v", resp["cookies"])
	}
	if d, _ := resp["data"].(map[string]any); d["id"] != float64(7) {
		t.Errorf("data: %v", resp["data"])
	}

	// Reopening shows the saved response.
	m.closeFile()
	m.openFileRaw(hit)
	if m.Response.Status != 201 {
		t.Errorf("saved response not shown on reopen: %d", m.Response.Status)
	}
}

// env.json edited outside the app (another editor, the terminal) between
// sends is picked up by the next send.
func TestSendPicksUpExternalEnvEdit(t *testing.T) {
	var mu sync.Mutex
	var auth []string
	m, hd, srv := apiProject(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auth = append(auth, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Write([]byte("ok"))
	})
	ch := withProgram(t)
	hit := filepath.Join(hd, "a.hit")
	writeHit(t, hit, hitfile.HitFile{Method: "GET", URL: "<<BASE_URL>>/me", Headers: map[string]string{"Authorization": "<<TOKEN>>"}})
	m.openFileRaw(hit)
	ctrl(m, tea.KeyCtrlR)
	awaitResponse(t, m, ch)

	b, _ := json.Marshal(map[string]string{"BASE_URL": srv.URL, "TOKEN": "rotated"})
	os.WriteFile(filepath.Join(hd, "env.json"), b, 0o644)
	ctrl(m, tea.KeyCtrlR)
	awaitResponse(t, m, ch)

	mu.Lock()
	defer mu.Unlock()
	if len(auth) != 2 || auth[0] != "s3cret" || auth[1] != "rotated" {
		t.Fatalf("auth headers seen: %v", auth)
	}
	if m.Response.Content != "ok" {
		t.Errorf("plain-text body should show verbatim: %q", m.Response.Content)
	}
}

// Editing env.json inside the app updates the variables used by the next
// send — no restart.
func TestSendUsesEnvEditedInApp(t *testing.T) {
	var mu sync.Mutex
	var got string
	m, hd, srv := apiProject(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = r.Header.Get("X-Key")
		mu.Unlock()
	})
	ch := withProgram(t)
	envPath := filepath.Join(hd, "env.json")
	m.openFileRaw(envPath)
	m.TextEd.SetContent(envPath, `{"BASE_URL":"`+srv.URL+`","TOKEN":"from-editor"}`)
	m.saveAndEnqueue()

	hit := filepath.Join(hd, "a.hit")
	writeHit(t, hit, hitfile.HitFile{Method: "GET", URL: "<<BASE_URL>>", Headers: map[string]string{"X-Key": "<<TOKEN>>"}})
	m.openFileRaw(hit)
	ctrl(m, tea.KeyCtrlR)
	awaitResponse(t, m, ch)
	mu.Lock()
	defer mu.Unlock()
	if got != "from-editor" {
		t.Fatalf("in-app env edit not used: %q", got)
	}
	m.WriteQueue.FlushNow()
	if raw, _ := os.ReadFile(envPath); !strings.Contains(string(raw), "from-editor") {
		t.Errorf("env.json not saved: %s", raw)
	}
}

// A 4xx/5xx is a response, not a failure: shown with its status and saved
// with ok=false.
func TestSendNon2xxIsShownNotFailed(t *testing.T) {
	for _, code := range []int{400, 404, 418, 500, 503} {
		m, hd, _ := apiProject(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			w.Write([]byte(`{"error":"nope"}`))
		})
		ch := withProgram(t)
		hit := filepath.Join(hd, "a.hit")
		writeHit(t, hit, hitfile.HitFile{Method: "GET", URL: "<<BASE_URL>>/x"})
		m.openFileRaw(hit)
		ctrl(m, tea.KeyCtrlR)
		awaitResponse(t, m, ch)
		if m.Response.Status != code || m.Response.Ok {
			t.Errorf("%d: shown as status=%d ok=%v", code, m.Response.Status, m.Response.Ok)
		}
		if strings.HasPrefix(m.StatusBar, "Request failed") {
			t.Errorf("%d reported as a failure: %q", code, m.StatusBar)
		}
		m.WriteQueue.FlushNow()
		if r, _ := readHit(t, hit)["response"].(map[string]any); r["status"] != float64(code) || r["ok"] != false {
			t.Errorf("%d persisted as %v", code, r)
		}
	}
}

// Nothing listening: the footer says the request failed, the spinner stops,
// the file keeps its previous response, and the user can send again.
func TestSendConnectionRefused(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close() // nothing listens here now

	m, hd := newTestScreen(t)
	ch := withProgram(t)
	hit := filepath.Join(hd, "a.hit")
	prev := &hitfile.Response{Status: 200, StatusText: "OK", Ok: true, Data: "old"}
	writeHit(t, hit, hitfile.HitFile{Method: "GET", URL: "http://" + addr + "/x", Response: prev})
	m.openFileRaw(hit)
	ctrl(m, tea.KeyCtrlR)
	awaitResponse(t, m, ch)
	if !strings.HasPrefix(m.StatusBar, "Request failed") {
		t.Errorf("status = %q", m.StatusBar)
	}
	if m.Sending {
		t.Error("spinner should stop after a failure")
	}
	m.WriteQueue.FlushNow()
	if r, _ := readHit(t, hit)["response"].(map[string]any); r["data"] != "old" {
		t.Errorf("a failed send must not wipe the previous response: %v", r)
	}
	if _, cmd := ctrl(m, tea.KeyCtrlR); cmd == nil || !m.Sending {
		t.Error("user should be able to retry")
	}
	awaitResponse(t, m, ch)
}

// A URL without a scheme gets http://; an empty URL is refused with a hint
// rather than sending.
func TestSendSchemelessAndEmptyURL(t *testing.T) {
	var hits int
	var mu sync.Mutex
	m, hd, srv := apiProject(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
	})
	ch := withProgram(t)
	hit := filepath.Join(hd, "a.hit")
	writeHit(t, hit, hitfile.HitFile{Method: "GET", URL: strings.TrimPrefix(srv.URL, "http://") + "/p"})
	m.openFileRaw(hit)
	ctrl(m, tea.KeyCtrlR)
	awaitResponse(t, m, ch)
	if m.Response.Status != 200 {
		t.Errorf("schemeless URL: status %d (%q)", m.Response.Status, m.StatusBar)
	}

	ctrl(m, tea.KeyCtrlU)
	typeText(m, "   ")
	if _, cmd := ctrl(m, tea.KeyCtrlR); cmd != nil || m.Sending {
		t.Error("blank URL must not send")
	}
	if m.StatusBar != "URL is empty" {
		t.Errorf("status = %q", m.StatusBar)
	}
	mu.Lock()
	defer mu.Unlock()
	if hits != 1 {
		t.Errorf("server hits = %d, want 1", hits)
	}
}

// Switching to another file while a request is in flight drops the response
// for the file it belonged to rather than writing it into the new one.
func TestResponseForAnotherFileIsNotMisfiled(t *testing.T) {
	release := make(chan struct{})
	m, hd, _ := apiProject(t, func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Write([]byte(`{"slow":true}`))
	})
	ch := withProgram(t)
	a := filepath.Join(hd, "a.hit")
	b := filepath.Join(hd, "b.hit")
	writeHit(t, a, hitfile.HitFile{Method: "GET", URL: "<<BASE_URL>>/slow"})
	writeHit(t, b, hitfile.HitFile{Method: "GET", URL: "<<BASE_URL>>/b"})
	m.openFileRaw(a)
	ctrl(m, tea.KeyCtrlR)
	m.openFileRaw(b)
	close(release)
	awaitResponse(t, m, ch)
	m.WriteQueue.FlushNow()
	if r := readHit(t, b)["response"]; r != nil {
		t.Errorf("b.hit got a's response: %v", r)
	}
	if m.Response.Status != 0 {
		t.Errorf("b's viewer shows a's response: %d", m.Response.Status)
	}
}

// ---------- Malformed input never crashes ----------

// Broken, empty, binary, huge or wrongly-typed .hit files and env.json open,
// render, toggle and "send" without panicking.
func TestMalformedFilesNeverPanic(t *testing.T) {
	huge := `{"method":"GET","url":"http://x","headers":{},"params":{},"body":"` + strings.Repeat("a", 3<<20) + `"}`
	cases := map[string]string{
		"empty":        "",
		"spaces":       "   \n\t",
		"not-json":     "GET http://example.com",
		"truncated":    `{"method":"GET","url":"http://x"`,
		"array":        `[1,2,3]`,
		"null":         `null`,
		"string":       `"just a string"`,
		"wrong-types":  `{"method":7,"url":["x"],"headers":"h","params":1,"body":{}}`,
		"number-hdrs":  `{"method":"GET","url":"http://x","headers":{"a":1}}`,
		"null-fields":  `{"method":null,"url":null,"headers":null,"params":null,"body":null,"response":null}`,
		"lower-method": `{"method":"get","url":"http://x"}`,
		"odd-response": `{"method":"GET","url":"http://x","response":{"status":"200","data":[{"deep":[[[]]]}]}}`,
		"binary":       "\x00\x01\x02\xff\xfe{\"method\":\"GET\"}",
		"bad-utf8":     "{\"method\":\"GET\",\"url\":\"http://x/\xff\xfe\"}",
		"huge":         huge,
		"unclosed-tpl": `{"method":"GET","url":"<<BASE_URL","body":"<<<<>>>>"}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic: %v", r)
				}
			}()
			m, hd := newTestScreen(t)
			p := filepath.Join(hd, name+".hit")
			os.WriteFile(p, []byte(content), 0o644)
			m.openFileRaw(p)
			if m.ActiveFile != p {
				t.Fatalf("file did not open (status %q)", m.StatusBar)
			}
			_ = m.View()
			ctrl(m, tea.KeyCtrlT)
			_ = m.View()
			ctrl(m, tea.KeyCtrlT)
			ctrl(m, tea.KeyCtrlR) // no program wired: must be a no-op, not a crash
			ctrl(m, tea.KeyCtrlY) // copy as curl interpolates the whole body
			ctrl(m, tea.KeyCtrlS)
			_ = m.View()
			m.closeFile()
		})
	}
}

// A broken env.json neither crashes startup nor sending; the request still
// runs with the template left as typed.
func TestMalformedEnvFile(t *testing.T) {
	for _, env := range []string{"", "{", "[]", `{"A":1}`, "null", "\x00\x00"} {
		t.Run(env, func(t *testing.T) {
			var mu sync.Mutex
			var path string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				path = r.URL.RawPath + r.URL.Path
				mu.Unlock()
			}))
			defer srv.Close()
			m, hd := newTestScreen(t)
			envPath := filepath.Join(hd, "env.json")
			os.WriteFile(envPath, []byte(env), 0o644)
			m = NewMainScreen(filepath.Dir(hd), m.Zones) // startup reads the broken file
			m.SetSize(120, 40)
			ch := withProgram(t)
			hit := filepath.Join(hd, "a.hit")
			writeHit(t, hit, hitfile.HitFile{Method: "GET", URL: srv.URL + "/v/<<BASE_URL>>"})
			m.openFileRaw(hit)
			ctrl(m, tea.KeyCtrlR)
			awaitResponse(t, m, ch)
			if m.Response.Status != 200 {
				t.Fatalf("status %d (%q)", m.Response.Status, m.StatusBar)
			}
			mu.Lock()
			defer mu.Unlock()
			if !strings.Contains(path, "BASE_URL") {
				t.Errorf("unresolved template should be sent as typed, server saw %q", path)
			}
			// Opening the broken env.json itself works too.
			m.openFileRaw(envPath)
			_ = m.View()
		})
	}
}

// ---------- Big content survives untouched ----------

// Opening a file and moving on must never rewrite it. The editor's buffer
// used to keep only the first 10000 rows, and switching files saved that cut
// buffer over the file — a lockfile or log lost everything past row 9999.
func TestLongFileSurvivesOpenAndSwitch(t *testing.T) {
	m, hd := newTestScreen(t)
	var sb strings.Builder
	for i := 0; i < 25000; i++ {
		sb.WriteString("row ")
		sb.WriteString(strings.Repeat("x", i%7))
		sb.WriteByte('\n')
	}
	want := sb.String()
	for _, name := range []string{"pnpm-lock.yaml", "big.md"} {
		p := filepath.Join(hd, name)
		os.WriteFile(p, []byte(want), 0o644)
		m.openFileRaw(p)
		if got := m.TextEd.GetContent(); got != want {
			t.Fatalf("%s: editor holds %d of %d rows", name, strings.Count(got, "\n"), 25000)
		}
		m.openFileRaw(filepath.Join(hd, "notes.md"))
		m.WriteQueue.FlushNow()
		if b, _ := os.ReadFile(p); string(b) != want {
			t.Fatalf("%s rewritten on switch: %d rows left", name, strings.Count(string(b), "\n"))
		}
	}

	// A .hit whose body is over 10000 rows: Body tab and Text view both keep it.
	hit := filepath.Join(hd, "bulk.hit")
	writeHit(t, hit, hitfile.HitFile{Method: "POST", URL: "http://x", Body: want})
	m.openFileRaw(hit)
	m.openFileRaw(filepath.Join(hd, "notes.md"))
	m.openFileRaw(hit)
	ctrl(m, tea.KeyCtrlT)
	ctrl(m, tea.KeyCtrlT)
	m.closeFile()
	m.WriteQueue.FlushNow()
	if got := readHit(t, hit)["body"]; got != want {
		t.Fatalf("bulk body cut to %d rows", strings.Count(got.(string), "\n"))
	}
}

// A long URL (big query string, signed URL, data: URI) is kept whole. The URL
// bar capped input at 2000 characters and saved the cut value on switch.
func TestLongURLSurvivesOpen(t *testing.T) {
	m, hd := newTestScreen(t)
	hit := filepath.Join(hd, "signed.hit")
	u := "https://bucket.example/obj?X-Signature=" + strings.Repeat("abcdef0123", 600)
	writeHit(t, hit, hitfile.HitFile{Method: "GET", URL: u})
	m.openFileRaw(hit)
	if _, got := m.URLBar.GetContent(); got != u {
		t.Fatalf("url bar holds %d of %d chars", len(got), len(u))
	}
	typeText(m, "&more=1") // typing past the old cap works too
	m.openFileRaw(filepath.Join(hd, "notes.md"))
	m.WriteQueue.FlushNow()
	if got := readHit(t, hit)["url"]; got != u+"&more=1" {
		t.Fatalf("saved url is %d chars, want %d", len(got.(string)), len(u)+7)
	}
}

// A request whose saved response is one long line (an HTML page, minified
// JSON) must switch to Text view and take edits promptly. The editor's line
// wrap was quadratic in the longest unbroken run: ~40s for 64KB.
func TestTextViewWithLongLineStaysResponsive(t *testing.T) {
	m, hd := newTestScreen(t)
	hit := filepath.Join(hd, "page.hit")
	page := "<html>" + strings.Repeat(`<div class="x">y</div>`, 12000) + "</html>" // ~270KB, no newline
	writeHit(t, hit, hitfile.HitFile{Method: "GET", URL: "http://x", Body: strings.Repeat("{}", 20000),
		Response: &hitfile.Response{Status: 200, StatusText: "OK", Ok: true, Data: page}})
	m.openFileRaw(hit)
	budget := 3 * time.Second * raceSlowdown
	start := time.Now()
	ctrl(m, tea.KeyCtrlT)
	if d := time.Since(start); d > budget || m.ViewMode != ViewText {
		t.Fatalf("ctrl+t took %s (view %d)", d, m.ViewMode)
	}
	// Walk onto the long line and type on it.
	start = time.Now()
	for i := 0; i < 12; i++ {
		ctrl(m, tea.KeyDown)
	}
	ctrl(m, tea.KeyEnd)
	for i := 0; i < 5; i++ {
		runes(m, "z")
	}
	_ = m.View()
	if d := time.Since(start); d > budget {
		t.Fatalf("moving and typing on the long line took %s", d)
	}
	// Back in Runner, typing into a long single-line body is just as quick.
	ctrl(m, tea.KeyCtrlT)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3"), Alt: true})
	start = time.Now()
	ctrl(m, tea.KeyEnd)
	for i := 0; i < 5; i++ {
		runes(m, "q")
	}
	if d := time.Since(start); d > budget {
		t.Fatalf("typing in a 40KB one-line body took %s", d)
	}
}
