package screens

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

func newTestScreen(t *testing.T) (*MainScreen, string) {
	t.Helper()
	tmp := t.TempDir()
	hd := filepath.Join(tmp, "hittable")
	os.MkdirAll(hd, 0o755)
	os.WriteFile(filepath.Join(hd, "env.json"), []byte(`{"BASE_URL":"http://x.test"}`), 0o644)
	os.WriteFile(filepath.Join(hd, "a.hit"), []byte(`{"method":"GET","url":"<<BASE_URL>>/a","headers":{},"params":{},"body":"","response":null}`), 0o644)
	os.WriteFile(filepath.Join(hd, "notes.md"), []byte("hello\n"), 0o644)
	m := NewMainScreen(tmp, zone.New())
	m.SetSize(120, 40)
	return m, hd
}

func readHit(t *testing.T, p string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var h map[string]any
	if err := json.Unmarshal(data, &h); err != nil {
		t.Fatalf("bad json in %s: %v\n%s", p, err, data)
	}
	return h
}

// Templates must survive open -> save; interpolation is send-time only.
func TestTemplatePreservedOnSave(t *testing.T) {
	m, hd := newTestScreen(t)
	hit := filepath.Join(hd, "a.hit")
	m.openFileRaw(hit)
	if _, u := m.URLBar.GetContent(); u != "<<BASE_URL>>/a" {
		t.Fatalf("url bar should show raw template, got %q", u)
	}
	m.Body.SetContent(`{"x":1}`)
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow()
	if got := readHit(t, hit)["url"]; got != "<<BASE_URL>>/a" {
		t.Errorf("template clobbered: %v", got)
	}
	if got := m.currentHit(); got == nil || !strings.Contains(m.Explorer.String(), "files") {
		t.Errorf("sanity: %v", got)
	}
}

// Edits made right before switching files must not be lost.
func TestSwitchingFilesKeepsPendingEdit(t *testing.T) {
	m, hd := newTestScreen(t)
	hit := filepath.Join(hd, "a.hit")
	md := filepath.Join(hd, "notes.md")
	m.openFileRaw(hit)
	m.Body.SetContent(`{"pending":true}`)
	m.openFileRaw(md) // no explicit save
	m.TextEd.SetContent(md, "edited\n")
	m.openFileRaw(hit) // re-open: must come from the store, not disk
	m.WriteQueue.FlushNow()
	if got := readHit(t, hit)["body"]; got != `{"pending":true}` {
		t.Errorf("hit edit lost: %v", got)
	}
	if got := m.Body.GetContent(); got != `{"pending":true}` {
		t.Errorf("reopen showed stale body: %q", got)
	}
	if b, _ := os.ReadFile(md); string(b) != "edited\n" {
		t.Errorf("md edit lost: %q", b)
	}
}

// Rename must rename (it used to create a new empty file), new folder must
// make a directory.
func TestRenameAndNewFolder(t *testing.T) {
	m, hd := newTestScreen(t)
	m.Explorer.Toggle(m.Explorer.Visible[0]) // expand hittable/
	var idx int
	for i, n := range m.Explorer.Visible {
		if n.Name == "a.hit" {
			idx = i
		}
	}
	m.Explorer.Cursor = idx
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	for _, r := range "\b\b\b\b\bb.hit" {
		if r == '\b' {
			m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		} else {
			m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected rename cmd")
	}
	m.Update(cmd())
	if _, err := os.Stat(filepath.Join(hd, "b.hit")); err != nil {
		t.Errorf("b.hit missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(hd, "a.hit")); err == nil {
		t.Errorf("a.hit still exists")
	}

	m.Explorer.BeginNewFolder()
	for _, r := range "sub" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(cmd())
	if fi, err := os.Stat(filepath.Join(hd, "sub")); err != nil || !fi.IsDir() {
		t.Errorf("sub/ not a directory: %v", err)
	}
}

// The rendered frame must be exactly Height rows in every UI state.
func TestLayoutFits(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}, {200, 60}} {
		m, hd := newTestScreen(t)
		m.SetSize(sz[0], sz[1])
		m.openFileRaw(filepath.Join(hd, "a.hit"))
		big := strings.Repeat(`{"k":"v"},`, 200)
		m.Response.SetResponse(200, "OK", 5, 10, true, "["+big[:len(big)-1]+"]")
		states := map[string]func(){
			"runner":      func() {},
			"dropdown":    func() { m.URLBar.ToggleDropdown() },
			"search":      func() { m.URLBar.CloseDropdown(); m.Response.OpenSearch() },
			"headers":     func() { m.Response.CloseSearch(); m.Response.ShowHeaders = true },
			"invalidjson": func() { m.Params.TextArea.SetValue("{bad"); m.Params.InvalidJSON = true },
			"help":        func() { m.ShowHelp = true },
			"text":        func() { m.ShowHelp = false; m.Params.SetContent(nil); m.toggleViewMode() },
			"menu":        func() { m.Explorer.OpenMenu() },
			"termopen":    func() { m.Explorer.MenuOpen = false; m.Term.Open = true; m.SetSize(sz[0], sz[1]) },
			"termrunner":  func() { m.toggleViewMode() },
			"git":         func() { m.Term.Open = false; m.SetSize(sz[0], sz[1]); m.GitOpen = true },
			"nosidebar":   func() { m.GitOpen = false; m.toggleExplorer() },
			"mdpreview":   func() { m.toggleExplorer(); m.openFileRaw(filepath.Join(hd, "notes.md")); m.setMdMode(MdPreview) },
			"mdsplit":     func() { m.setMdMode(MdSplit) },
		}
		for _, name := range []string{"runner", "dropdown", "search", "headers", "invalidjson", "help", "text", "menu", "termopen", "termrunner", "git", "nosidebar", "mdpreview", "mdsplit"} {
			states[name]()
			lines := strings.Split(m.View(), "\n")
			if len(lines) != sz[1] {
				t.Errorf("%v %s: got %d rows, want %d", sz, name, len(lines), sz[1])
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w > sz[0] {
					t.Errorf("%v %s: row %d is %d wide", sz, name, i, w)
					break
				}
			}
		}
	}
}

// Highlighting is computed once per response, not per frame.
func TestResponseViewIsCheap(t *testing.T) {
	m, hd := newTestScreen(t)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < 20000; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"id":` + strconv.Itoa(i) + `,"name":"item","tags":["a","b"],"ok":true}`)
	}
	sb.WriteString("]")
	start := time.Now()
	m.Response.SetResponse(200, "OK", 1, sb.Len(), true, sb.String())
	setCost := time.Since(start)
	start = time.Now()
	for i := 0; i < 50; i++ {
		m.Response.Scroll(false)
		_ = m.View()
	}
	perFrame := time.Since(start) / 50
	t.Logf("SetResponse %s (%.1f MB), View %s/frame", setCost, float64(sb.Len())/1e6, perFrame)
	if perFrame > 20*time.Millisecond {
		t.Errorf("View too slow: %s per frame", perFrame)
	}
}

// Mouse drives the runner: click places the cursor in a tab editor, drag
// selects, the Send button sends, the response label toggles headers.
func TestMouseInRunner(t *testing.T) {
	m, hd := newTestScreen(t)
	m.SetSize(120, 40)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	m.Body.SetContent("hello world\nsecond line")
	m.ActiveTab = 2
	scan := func() { m.Zones.Scan(m.View()); time.Sleep(30 * time.Millisecond) } // zones register async
	scan()
	z := m.Zones.Get("editor")
	if z == nil || z.IsZero() {
		t.Fatal("editor zone not registered")
	}
	gutter := m.Body.GutterWidth() // asked for, not assumed
	x0, y0 := z.StartX+1+gutter, z.StartY+1
	press := func(x, y int) {
		m.Update(tea.MouseMsg{X: x, Y: y, Type: tea.MouseLeft, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	}
	press(x0+6, y0) // "world"
	if m.Focus != FocusBody || m.Body.GetCursorRow() != 0 || m.Body.TextArea.LineInfo().ColumnOffset != 6 {
		t.Fatalf("click: focus=%v row=%d col=%d", m.Focus, m.Body.GetCursorRow(), m.Body.TextArea.LineInfo().ColumnOffset)
	}
	m.Update(tea.MouseMsg{X: x0 + 11, Y: y0, Type: tea.MouseLeft, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft})
	if got := m.Body.SelectedText(); got != "world" {
		t.Errorf("drag selection = %q", got)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("there")})
	if got := m.Body.GetContent(); !strings.HasPrefix(got, "hello there") {
		t.Errorf("typing should replace selection: %q", got)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if got := m.Body.GetContent(); !strings.HasPrefix(got, "hello world") {
		t.Errorf("undo after replace: %q", got)
	}

	m.URLBar.SetContent("GET", "")
	scan()
	sz := m.Zones.Get("send_btn")
	press(sz.StartX+1, sz.StartY)
	if m.StatusBar != "URL is empty" {
		t.Errorf("send button did not trigger send: %q", m.StatusBar)
	}

	m.Response.SetResponse(200, "OK", 1, 2, true, `{}`)
	scan()
	rz := m.Zones.Get("resp_mode")
	press(rz.StartX, rz.StartY)
	if !m.Response.ShowHeaders {
		t.Error("clicking the body/headers label should toggle headers view")
	}
	press(x0, y0) // click back into the editor: drops explorer focus, focuses body
	if m.ExplorerFocused || m.Focus != FocusBody {
		t.Errorf("focus after editor click: explorer=%v focus=%v", m.ExplorerFocused, m.Focus)
	}
}

// Git panel end to end on a temp repo: open, stage, commit, decorations.
func TestGitPanel(t *testing.T) {
	m, hd := newTestScreen(t)
	root := filepath.Dir(hd)
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@x")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("config", "user.email", "t@x")
	git("config", "user.name", "T")
	git("add", ".")
	git("commit", "-q", "-m", "init")
	os.WriteFile(filepath.Join(hd, "notes.md"), []byte("hello\nchanged\n"), 0o644)

	m = NewMainScreen(root, m.Zones) // re-open now that it is a repo
	m.SetSize(120, 40)
	if m.Repo == nil {
		t.Fatal("repo not detected")
	}
	m.Explorer.Toggle(m.Explorer.Visible[0])
	if b := m.Explorer.GitStatus[filepath.Join(hd, "notes.md")]; b != "M" {
		t.Errorf("explorer badge = %q, want M", b)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyF5})
	if !m.GitOpen {
		t.Fatal("F5 should open the git panel")
	}
	view := m.View()
	if !strings.Contains(view, "Changes (1)") || !strings.Contains(view, "notes.md") || !strings.Contains(view, "+changed") {
		t.Fatalf("status view:\n%s", view)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyDown}) // onto the file row
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if !strings.Contains(m.View(), "Staged Changes (1)") {
		t.Fatalf("stage failed:\n%s", m.View())
	}
	// c opens the compose editor pre-filled with the heuristic draft. This
	// repo's one commit ("init") is not conventional, so the type picker is
	// skipped. Select all and type over the draft, then ctrl+s to commit.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if !m.Git.Composing {
		t.Fatalf("c should open the compose editor:\n%s", m.View())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	for _, r := range "add greeting" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if v := m.View(); !strings.Contains(v, "Changes (0)") || strings.Contains(v, "M hittable/notes.md") {
		t.Fatalf("commit failed:\n%s", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}) // commits
	if v := m.View(); !strings.Contains(v, "add greeting") || !strings.Contains(v, "init") {
		t.Fatalf("commits view:\n%s", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.GitOpen {
		t.Error("esc should close the panel")
	}

	// Inline blame in the editor.
	m.openFileRaw(filepath.Join(hd, "notes.md"))
	m.Update(tea.KeyMsg{Type: tea.KeyF5})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	if v := m.View(); !strings.Contains(v, "add greeting") {
		t.Fatalf("blame view:\n%s", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v := m.View(); !m.BlameOn || !strings.Contains(v, "add greeting") {
		t.Fatalf("inline blame missing:\n%s", v)
	}
}

// Navbar panels are exclusive: opening one closes the others.
func TestNavbarPanelsExclusive(t *testing.T) {
	m, _ := newTestScreen(t)
	m.Update(tea.KeyMsg{Type: tea.KeyF5})    // git
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP}) // find
	if m.GitOpen || !m.Palette.Open {
		t.Errorf("find should close git: git=%v find=%v", m.GitOpen, m.Palette.Open)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ}) // terminal
	if !m.Term.Open || m.Palette.Open {
		t.Errorf("terminal should be open alone: term=%v find=%v", m.Term.Open, m.Palette.Open)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyF5})
	if !m.GitOpen || m.Term.Open || m.TermFocused {
		t.Errorf("git should close terminal: git=%v term=%v focused=%v", m.GitOpen, m.Term.Open, m.TermFocused)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyF1})
	if !m.ShowHelp || m.GitOpen {
		t.Errorf("help should close git: help=%v git=%v", m.ShowHelp, m.GitOpen)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyF1})
	if m.ShowHelp {
		t.Error("second F1 should close help")
	}
}

// Merge conflict: resolve a block in the panel, mark resolved, commit the merge.
func TestMergeConflictInPanel(t *testing.T) {
	m, hd := newTestScreen(t)
	root := filepath.Dir(hd)
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@x")
		out, _ := cmd.CombinedOutput()
		return string(out)
	}
	git("init", "-q", "-b", "main")
	git("config", "user.email", "t@x")
	git("config", "user.name", "T")
	git("add", ".")
	git("commit", "-q", "-m", "init")
	git("checkout", "-q", "-b", "feature")
	os.WriteFile(filepath.Join(hd, "notes.md"), []byte("theirs\n"), 0o644)
	git("commit", "-q", "-am", "feature")
	git("checkout", "-q", "main")
	os.WriteFile(filepath.Join(hd, "notes.md"), []byte("ours\n"), 0o644)
	git("commit", "-q", "-am", "main")
	git("merge", "feature") // conflicts

	m = NewMainScreen(root, m.Zones)
	m.SetSize(120, 40)
	if b := m.Explorer.GitStatus[filepath.Join(hd, "notes.md")]; b != "!" {
		t.Errorf("conflict badge = %q", b)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyF5})
	v := m.View()
	if !strings.Contains(v, "merge in progress · 1 conflict") || !strings.Contains(v, "abort merge") {
		t.Fatalf("merge header missing:\n%s", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // cursor sits on the conflicted file
	if !m.Git.Resolving || len(m.Git.Conflicts) != 1 {
		t.Fatalf("resolve mode: %v %d", m.Git.Resolving, len(m.Git.Conflicts))
	}
	if v := m.View(); !strings.Contains(v, "conflict 1/1") || !strings.Contains(v, "<<<<<<<") {
		t.Fatalf("resolver view:\n%s", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")}) // accept both
	if len(m.Git.Conflicts) != 0 {
		t.Fatalf("conflict not resolved")
	}
	b, _ := os.ReadFile(filepath.Join(hd, "notes.md"))
	if string(b) != "ours\ntheirs\n" {
		t.Errorf("file after accept both: %q", b)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}) // mark resolved
	v = m.View()
	if m.Git.Resolving || !strings.Contains(v, "0 conflict(s)") || !strings.Contains(v, "commit merge") {
		t.Fatalf("after mark resolved:\n%s", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")}) // top: the merge header (it has a button)
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})                     // header action: commit merge
	if m.Git.Merge {
		t.Errorf("merge should be finished: %s", m.View())
	}
	if log := git("log", "--oneline", "-1"); !strings.Contains(log, "Merge") {
		t.Errorf("merge commit missing: %s", log)
	}
}

func TestExplorerToggle(t *testing.T) {
	m, hd := newTestScreen(t)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b"), Alt: true})
	if !m.ExplorerHidden || m.MainWidth != m.Width || strings.Contains(m.View(), "/001") {
		t.Fatalf("explorer should be hidden: hidden=%v main=%d", m.ExplorerHidden, m.MainWidth)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlB}) // ctrl+b unhides and focuses
	if m.ExplorerHidden || !m.ExplorerFocused {
		t.Errorf("ctrl+b should show and focus the explorer")
	}
	if m.Git.SyncLabel() == "" {
		t.Error("sync label empty")
	}
}

// Markdown: Text → Preview → Split via ctrl+t, mermaid rendered, toggle clickable.
func TestMarkdownPreviewModes(t *testing.T) {
	m, hd := newTestScreen(t)
	md := filepath.Join(hd, "doc.md")
	os.WriteFile(md, []byte("# Hello World\n\nSome *text*.\n\n```mermaid\ngraph LR\n  A[Start] --> B[Finish]\n```\n"), 0o644)
	m.SetSize(140, 40)
	m.openFileRaw(md)
	if !m.isMarkdown() || m.MdMode != MdText {
		t.Fatalf("md should open in text mode: md=%v mode=%d", m.isMarkdown(), m.MdMode)
	}
	if v := m.View(); !strings.Contains(v, "[ Text | Preview | Split ]") && !strings.Contains(v, "Preview") {
		t.Fatalf("toggle missing:\n%s", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	if m.MdMode != MdPreview || m.Focus != FocusPreview {
		t.Fatalf("ctrl+t should enter preview: mode=%d focus=%d", m.MdMode, m.Focus)
	}
	v := stripAnsi(m.View())
	if !strings.Contains(v, "Hello World") || strings.Contains(v, "*text*") {
		t.Errorf("preview should render markdown (emphasis markers gone):\n%s", v)
	}
	if strings.Contains(v, "graph LR") || !strings.Contains(v, "Start") || !strings.Contains(v, "Finish") {
		t.Errorf("mermaid should be rendered as a diagram:\n%s", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	if m.MdMode != MdSplit || m.Focus != FocusTextEditor {
		t.Fatalf("second ctrl+t should enter split: mode=%d focus=%d", m.MdMode, m.Focus)
	}
	v = stripAnsi(m.View())
	if !strings.Contains(v, "# Hello World") || !strings.Contains(v, "Finish") {
		t.Errorf("split should show editor and preview:\n%s", v)
	}
	// Typing in split updates the preview.
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for _, r := range "## Added" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if v := stripAnsi(m.View()); strings.Count(v, "Added") < 2 {
		t.Errorf("preview did not follow the edit:\n%s", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	if m.MdMode != MdText {
		t.Errorf("third ctrl+t should return to text")
	}
	// .hit files keep the runner/text toggle on ctrl+t.
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	if m.ViewMode != ViewText {
		t.Error("ctrl+t on .hit should toggle text view")
	}
}

func stripAnsi(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			in = true
		case in && (r == 'm' || r == 'z'):
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
