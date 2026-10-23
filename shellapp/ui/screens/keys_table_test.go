package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/ui/components/palette"
	"github.com/hittable/shellapp/ui/components/requesteditor"
)

// Rows of the keybinding table in shellapp/CLAUDE.md §9 / README, each
// driven through the screen the way a user presses it.

func alt(m *MainScreen, r string) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(r), Alt: true})
}

func fkey(m *MainScreen, k tea.KeyType) { m.Update(tea.KeyMsg{Type: k}) }

// fakeShell makes any terminal start spawn a harmless process.
func fakeShell(t *testing.T) {
	t.Helper()
	sh := filepath.Join(t.TempDir(), "sh")
	os.WriteFile(sh, []byte("#!/bin/sh\nsleep 30\n"), 0o755)
	t.Setenv("SHELL", sh)
}

// Quit: ctrl+c with nothing selected quits, and an unsaved edit is handed to
// the write queue first.
func TestKeyQuitSavesFirst(t *testing.T) {
	m, hd := newTestScreen(t)
	hit := filepath.Join(hd, "a.hit")
	m.openFileRaw(hit)
	m.Body.SetContent("typed just before quitting")
	if _, cmd := ctrl(m, tea.KeyCtrlC); !isQuit(cmd) {
		t.Fatal("ctrl+c should quit")
	}
	m.WriteQueue.FlushNow() // what App.Cleanup does after Run returns
	if readHit(t, hit)["body"] != "typed just before quitting" {
		t.Error("the last edit was lost on quit")
	}
	// From the palette, help and git too.
	for name, open := range map[string]func(){
		"palette": func() { m.openPalette(palette.ModeFiles) },
		"help":    func() { m.toggleHelp() },
		"git":     func() { m.toggleGit() },
	} {
		open()
		if _, cmd := ctrl(m, tea.KeyCtrlC); !isQuit(cmd) {
			t.Errorf("ctrl+c should quit from %s", name)
		}
		m.closePanels()
	}
}

// Help: ? (explorer) and F1 (anywhere) toggle it; ↑↓ scroll; esc / q close.
func TestKeyHelp(t *testing.T) {
	m, hd := newTestScreen(t)
	m.SetSize(80, 20) // small enough that the reference scrolls
	runes(m, "?")
	if !m.ShowHelp {
		t.Fatal("? in the explorer opens help")
	}
	_ = m.View()
	ctrl(m, tea.KeyDown)
	ctrl(m, tea.KeyDown)
	if m.HelpScroll != 2 {
		t.Errorf("↓ scrolls help, scroll=%d", m.HelpScroll)
	}
	runes(m, "G")
	runes(m, "g")
	if m.HelpScroll != 0 {
		t.Errorf("g returns to the top, scroll=%d", m.HelpScroll)
	}
	runes(m, "q")
	if m.ShowHelp {
		t.Fatal("q closes help")
	}
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	runes(m, "?") // typing ? into the URL bar must not open help
	if m.ShowHelp {
		t.Fatal("? typed into the URL bar opened help")
	}
	if _, u := m.URLBar.GetContent(); !strings.HasSuffix(u, "?") {
		t.Errorf("? should be typed into the URL, got %q", u)
	}
	fkey(m, tea.KeyF1)
	if !m.ShowHelp {
		t.Fatal("F1 opens help from anywhere")
	}
	runes(m, "x") // swallowed while help is up
	ctrl(m, tea.KeyEsc)
	if m.ShowHelp || m.ActiveFile == "" {
		t.Fatal("esc closes help, not the file")
	}
}

// ctrl+b moves between the explorer and the main pane and back to where the
// user was; alt+b hides and shows the sidebar.
func TestKeyExplorerToggle(t *testing.T) {
	m, hd := newTestScreen(t)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	ctrl(m, tea.KeyTab) // to the tab bar
	ctrl(m, tea.KeyCtrlB)
	if !m.ExplorerFocused {
		t.Fatal("ctrl+b focuses the explorer")
	}
	ctrl(m, tea.KeyCtrlB)
	if m.ExplorerFocused || m.Focus != FocusTabBar {
		t.Fatalf("ctrl+b again returns to the tab bar, focus=%d", m.Focus)
	}
	alt(m, "b")
	if !m.ExplorerHidden || m.MainWidth != m.Width {
		t.Fatal("alt+b hides the sidebar and gives the main pane the width")
	}
	ctrl(m, tea.KeyCtrlB)
	if m.ExplorerHidden || !m.ExplorerFocused {
		t.Fatal("ctrl+b with the sidebar hidden shows and focuses it")
	}
	// With no file open, ctrl+b keeps the explorer (nothing to go back to).
	m.closeFile()
	ctrl(m, tea.KeyCtrlB)
	if !m.ExplorerFocused {
		t.Error("ctrl+b with no file open should stay in the explorer")
	}
}

// tab / shift+tab cycle URL → tabs → editor → response in Runner, and the
// editor ⇄ explorer in Text view; shift+tab in an editor leaves it.
func TestKeyTabCycle(t *testing.T) {
	m, hd := newTestScreen(t)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	want := []FocusArea{FocusTabBar, FocusBody, FocusResponse, FocusURLBar}
	for i, f := range want {
		ctrl(m, tea.KeyTab)
		if m.Focus != f {
			t.Fatalf("tab #%d: focus %d, want %d", i+1, m.Focus, f)
		}
	}
	ctrl(m, tea.KeyShiftTab)
	if m.Focus != FocusResponse {
		t.Fatalf("shift+tab goes back, focus %d", m.Focus)
	}
	m.openFileRaw(filepath.Join(hd, "notes.md"))
	ctrl(m, tea.KeyTab)
	if got := m.TextEd.GetContent(); !strings.HasPrefix(got, "  ") || m.ExplorerFocused {
		t.Fatalf("tab in the text editor indents two spaces: %q", got)
	}
	ctrl(m, tea.KeyShiftTab)
	if !m.ExplorerFocused {
		t.Fatal("shift+tab leaves the editor for the explorer")
	}
	ctrl(m, tea.KeyTab)
	if m.ExplorerFocused || m.Focus != FocusTextEditor {
		t.Fatal("tab in the explorer goes back to the open file")
	}
}

// Tab bar: ←/→ and h/l wrap around, 1/2/3 pick.
func TestKeyTabBar(t *testing.T) {
	m, hd := newTestScreen(t)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	ctrl(m, tea.KeyTab)
	ctrl(m, tea.KeyLeft)
	if m.ActiveTab != requesteditor.TabBody {
		t.Errorf("← from Params wraps to Body, got %d", m.ActiveTab)
	}
	runes(m, "l")
	if m.ActiveTab != requesteditor.TabParams {
		t.Errorf("l wraps to Params, got %d", m.ActiveTab)
	}
	ctrl(m, tea.KeyRight)
	runes(m, "h")
	runes(m, "3")
	if m.ActiveTab != requesteditor.TabBody {
		t.Errorf("3 picks Body, got %d", m.ActiveTab)
	}
	ctrl(m, tea.KeyUp)
	if m.Focus != FocusURLBar {
		t.Errorf("↑ from the tab bar goes to the URL bar, focus %d", m.Focus)
	}
	alt(m, "2")
	if m.ActiveTab != requesteditor.TabHeaders || m.Focus != FocusBody {
		t.Errorf("alt+2: tab %d focus %d", m.ActiveTab, m.Focus)
	}
}

// Response: ctrl+f searches, enter / shift+enter walk the matches, esc closes
// the search (not the file); h shows headers; g/G and pgdn scroll; ctrl+y
// copies the body.
func TestKeyResponsePane(t *testing.T) {
	m, hd := newTestScreen(t)
	m.SetSize(120, 30)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	var body strings.Builder
	body.WriteString("[\n")
	for i := 0; i < 200; i++ {
		body.WriteString(`  {"item": "row"},` + "\n")
	}
	body.WriteString(`  {"needle": "here"}` + "\n]")
	m.Response.SetResponse(200, "OK", 5, body.Len(), true, body.String())
	m.Response.SetHeaders(map[string]string{"X-Trace": "abc"})

	ctrl(m, tea.KeyCtrlF)
	if !m.Response.SearchOpen || m.Focus != FocusResponse {
		t.Fatal("ctrl+f opens response search")
	}
	typeText(m, "row")
	if len(m.Response.SearchMatches) != 200 {
		t.Fatalf("matches = %d", len(m.Response.SearchMatches))
	}
	first := m.Response.SearchIdx
	ctrl(m, tea.KeyEnter)
	if m.Response.SearchIdx != (first+1)%200 {
		t.Errorf("enter → next match: %d → %d", first, m.Response.SearchIdx)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyUp}) // up = previous, like shift+enter
	if m.Response.SearchIdx != first {
		t.Errorf("previous match: %d", m.Response.SearchIdx)
	}
	ctrl(m, tea.KeyEsc)
	if m.Response.SearchOpen || m.ActiveFile == "" {
		t.Fatal("esc closes the search only")
	}

	runes(m, "G")
	if m.Response.ScrollY == 0 {
		t.Error("G scrolls to the bottom")
	}
	runes(m, "g")
	if m.Response.ScrollY != 0 {
		t.Error("g scrolls to the top")
	}
	ctrl(m, tea.KeyPgDown)
	if m.Response.ScrollY == 0 {
		t.Error("pgdn scrolls")
	}
	runes(m, "h")
	if !m.Response.ShowHeaders || !strings.Contains(m.View(), "X-Trace") {
		t.Error("h shows the response headers")
	}
	runes(m, "h")
	ctrl(m, tea.KeyCtrlY)
	if m.StatusBar != "Response body copied" && m.StatusBar != "Clipboard unavailable" {
		t.Errorf("ctrl+y in the response: %q", m.StatusBar)
	}
}

// ctrl+y outside the response copies the request as curl, with env values
// resolved (and shows the command when there is no clipboard).
func TestKeyCopyAsCurl(t *testing.T) {
	m, hd := newTestScreen(t)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	ctrl(m, tea.KeyCtrlY)
	if m.StatusBar != "Copied as curl" && !strings.Contains(m.StatusBar, "curl -X GET") {
		t.Fatalf("ctrl+y: %q", m.StatusBar)
	}
	if strings.HasPrefix(m.StatusBar, "curl ") && !strings.Contains(m.StatusBar, "http://x.test/a") {
		t.Errorf("curl should resolve <<BASE_URL>>: %q", m.StatusBar)
	}
}

// Text editor: undo / redo, go-to-line, find, fold, wrap.
func TestKeyTextEditor(t *testing.T) {
	m, hd := newTestScreen(t)
	md := filepath.Join(hd, "notes.md")
	os.WriteFile(md, []byte("one\ntwo\nthree\n  four\n  five\n"), 0o644)
	m.openFileRaw(md)

	runes(m, "X")
	if !strings.HasPrefix(m.TextEd.GetContent(), "Xone") {
		t.Fatalf("typing: %q", m.TextEd.GetContent())
	}
	ctrl(m, tea.KeyCtrlZ)
	if strings.HasPrefix(m.TextEd.GetContent(), "X") {
		t.Fatal("ctrl+z undoes")
	}
	ctrl(m, tea.KeyCtrlY)
	if !strings.HasPrefix(m.TextEd.GetContent(), "Xone") {
		t.Fatal("ctrl+y redoes in the editor (not copy-as-curl)")
	}

	ctrl(m, tea.KeyCtrlG) // go to line — not the git panel inside an editor
	if m.GitOpen {
		t.Fatal("ctrl+g in an editor opened git")
	}
	runes(m, "3")
	ctrl(m, tea.KeyEnter)
	if m.TextEd.GetCursorRow() != 2 {
		t.Fatalf("go to line 3: row %d", m.TextEd.GetCursorRow())
	}

	ctrl(m, tea.KeyCtrlF)
	runes(m, "five")
	ctrl(m, tea.KeyEnter)
	ctrl(m, tea.KeyEsc) // closes the find prompt, not the file
	if m.TextEd.GetCursorRow() != 4 || m.ActiveFile == "" {
		t.Fatalf("find 'five': row %d active=%q", m.TextEd.GetCursorRow(), m.ActiveFile)
	}

	m.TextEd.GotoLine(2) // "three" heads the indented block
	ctrl(m, tea.KeyCtrlO)
	if !strings.Contains(m.View(), "▸") {
		t.Error("ctrl+o should fold the block")
	}
	wrap := m.TextEd.Wrap
	alt(m, "z")
	if m.TextEd.Wrap == wrap {
		t.Error("alt+z toggles wrap")
	}
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow()
	if b, _ := os.ReadFile(md); !strings.HasPrefix(string(b), "Xone") {
		t.Errorf("editor edit not saved: %q", b)
	}
}

// Markdown: ctrl+t cycles Text → Preview → Split → Text; tab moves between
// editor and preview in Split; the preview scrolls with j/k.
func TestKeyMarkdownModes(t *testing.T) {
	m, hd := newTestScreen(t)
	md := filepath.Join(hd, "notes.md")
	os.WriteFile(md, []byte("# Title\n\n"+strings.Repeat("para\n\n", 80)), 0o644)
	m.openFileRaw(md)
	ctrl(m, tea.KeyCtrlT)
	if m.MdMode != MdPreview || m.Focus != FocusPreview {
		t.Fatalf("ctrl+t → Preview: mode %d focus %d", m.MdMode, m.Focus)
	}
	_ = m.View()
	runes(m, "j")
	runes(m, "j")
	if m.Preview.ScrollY != 2 {
		t.Errorf("j scrolls the preview: %d", m.Preview.ScrollY)
	}
	ctrl(m, tea.KeyCtrlT)
	if m.MdMode != MdSplit || m.Focus != FocusTextEditor {
		t.Fatalf("ctrl+t → Split: mode %d focus %d", m.MdMode, m.Focus)
	}
	ctrl(m, tea.KeyShiftTab)
	if m.Focus != FocusPreview {
		t.Fatalf("shift+tab in split goes to the preview, focus %d", m.Focus)
	}
	ctrl(m, tea.KeyTab)
	if m.Focus != FocusTextEditor {
		t.Fatalf("tab in split returns to the editor, focus %d", m.Focus)
	}
	ctrl(m, tea.KeyCtrlT)
	if m.MdMode != MdText {
		t.Fatalf("ctrl+t → Text: mode %d", m.MdMode)
	}
	// The choice is remembered for the next markdown file.
	ctrl(m, tea.KeyCtrlT)
	other := filepath.Join(hd, "other.md")
	os.WriteFile(other, []byte("# Other\n"), 0o644)
	m.openFileRaw(other)
	if m.MdMode != MdPreview {
		t.Errorf("markdown mode not remembered: %d", m.MdMode)
	}
}

// Git panel opens from ctrl+g (outside editors), alt+g, F5 and g in the
// explorer, and each toggles it closed. In a folder that is not a repository
// it says so instead of failing.
func TestKeyGitPanelBindings(t *testing.T) {
	m, _ := newTestScreen(t)
	for name, press := range map[string]func(){
		"ctrl+g": func() { ctrl(m, tea.KeyCtrlG) },
		"alt+g":  func() { alt(m, "g") },
		"f5":     func() { fkey(m, tea.KeyF5) },
	} {
		press()
		if !m.GitOpen {
			t.Fatalf("%s should open git", name)
		}
		if !strings.Contains(m.View(), "Not a git repository") {
			t.Errorf("%s: no repository message", name)
		}
		if name == "ctrl+g" {
			ctrl(m, tea.KeyCtrlG)
		} else {
			press()
		}
		if m.GitOpen {
			t.Fatalf("%s again should close git", name)
		}
	}
	m.setExplorerFocused(true)
	runes(m, "g")
	if !m.GitOpen {
		t.Fatal("g in the explorer opens git")
	}
}

// Find file: ctrl+p or / opens the palette (sidebar out of the way), typing
// filters fuzzily, enter opens the file and brings the sidebar back; esc and
// ctrl+p close it.
func TestKeyFindFilePalette(t *testing.T) {
	m, hd := newTestScreen(t)
	os.MkdirAll(filepath.Join(hd, "node_modules", "pkg"), 0o755)
	os.WriteFile(filepath.Join(hd, "node_modules", "pkg", "notes.md"), []byte("vendored"), 0o644)
	ctrl(m, tea.KeyCtrlP)
	if !m.Palette.Open || m.Palette.Mode != palette.ModeFiles {
		t.Fatal("ctrl+p opens find file")
	}
	if !m.explorerHidden() || m.MainWidth != m.Width {
		t.Error("the palette takes the full width")
	}
	typeText(m, "nts")
	if len(m.Palette.Results) == 0 || !strings.HasSuffix(m.Palette.Results[0].Rel, "notes.md") {
		t.Fatalf("fuzzy results: %+v", m.Palette.Results)
	}
	for _, r := range m.Palette.Results {
		if strings.Contains(r.Rel, "node_modules") {
			t.Errorf("node_modules should be skipped: %s", r.Rel)
		}
	}
	ctrl(m, tea.KeyEnter)
	if m.Palette.Open || m.ActiveFile != filepath.Join(hd, "notes.md") {
		t.Fatalf("enter opens the file: open=%v active=%s", m.Palette.Open, m.ActiveFile)
	}
	if m.explorerHidden() {
		t.Error("the sidebar comes back after the palette closes")
	}

	m.setExplorerFocused(true)
	runes(m, "/")
	if !m.Palette.Open {
		t.Fatal("/ in the explorer opens find file")
	}
	ctrl(m, tea.KeyEsc)
	if m.Palette.Open || m.ActiveFile == "" {
		t.Fatal("esc closes the palette and keeps the file")
	}
	ctrl(m, tea.KeyCtrlP)
	ctrl(m, tea.KeyCtrlP)
	if m.Palette.Open {
		t.Fatal("ctrl+p toggles the palette closed")
	}
	// A query with no match shows nothing and enter does nothing.
	ctrl(m, tea.KeyCtrlP)
	typeText(m, "zzzqqq")
	ctrl(m, tea.KeyEnter)
	if len(m.Palette.Results) != 0 || !m.Palette.Open {
		t.Errorf("no-match enter: results=%d open=%v", len(m.Palette.Results), m.Palette.Open)
	}
}

// Live grep: alt+f searches file contents; the result opens at its line —
// a .hit switches to Text view so the line exists. tab flips back to files.
func TestKeyLiveGrepOpensAtLine(t *testing.T) {
	m, hd := newTestScreen(t)
	ch := withProgram(t)
	os.WriteFile(filepath.Join(hd, "deep.md"), []byte("a\nb\nc\nthe grep target\n"), 0o644)
	alt(m, "f")
	if !m.Palette.Open || m.Palette.Mode != palette.ModeGrep {
		t.Fatal("alt+f opens live grep")
	}
	typeText(m, "grep target")
	deadline := time.After(10 * time.Second)
	for len(m.Palette.Results) == 0 {
		select {
		case msg := <-ch:
			if r, ok := msg.(palette.ResultsMsg); ok {
				m.Update(r)
			}
		case <-deadline:
			t.Fatal("no grep results")
		}
	}
	r := m.Palette.Results[0]
	if !strings.HasSuffix(r.Path, "deep.md") || r.Line != 4 {
		t.Fatalf("grep result %+v", r)
	}
	ctrl(m, tea.KeyEnter)
	if m.ActiveFile != r.Path || m.TextEd.GetCursorRow() != 3 || m.Focus != FocusTextEditor {
		t.Fatalf("open at line: active=%s row=%d focus=%d", m.ActiveFile, m.TextEd.GetCursorRow(), m.Focus)
	}

	// Grep into a .hit: it opens in Text view at the matching line.
	alt(m, "f")
	typeText(m, "BASE_URL>>/a")
	deadline = time.After(10 * time.Second)
	for len(m.Palette.Results) == 0 {
		select {
		case msg := <-ch:
			if r, ok := msg.(palette.ResultsMsg); ok {
				m.Update(r)
			}
		case <-deadline:
			t.Fatal("no grep results for the .hit")
		}
	}
	ctrl(m, tea.KeyEnter)
	if !strings.HasSuffix(m.ActiveFile, "a.hit") || m.ViewMode != ViewText {
		t.Fatalf("grep into a .hit: active=%s view=%d", m.ActiveFile, m.ViewMode)
	}

	alt(m, "f")
	ctrl(m, tea.KeyTab)
	if m.Palette.Mode != palette.ModeFiles {
		t.Error("tab flips the palette to find file")
	}
	alt(m, "f")
	alt(m, "f")
	if m.Palette.Open {
		t.Error("alt+f toggles grep closed")
	}
}

// Terminal: ctrl+j opens and focuses it, ctrl+b leaves to the explorer,
// ctrl+j again focuses, and once more hides it. Keys typed while focused go
// to the shell, not the app.
func TestKeyTerminalToggle(t *testing.T) {
	fakeShell(t)
	m, hd := newTestScreen(t)
	t.Cleanup(m.Term.Close)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	ctrl(m, tea.KeyCtrlJ)
	if !m.Term.Open || !m.TermFocused {
		t.Fatalf("ctrl+j opens the terminal: open=%v focused=%v (%q)", m.Term.Open, m.TermFocused, m.StatusBar)
	}
	if m.MainH+1+m.Term.Rows+2 != m.Height {
		t.Errorf("terminal layout: main %d + term %d != %d", m.MainH, m.Term.Rows, m.Height)
	}
	runes(m, "?")
	ctrl(m, tea.KeyCtrlT)
	ctrl(m, tea.KeyCtrlS)
	if m.ShowHelp || m.ViewMode != ViewRunner || m.StatusBar == "Saved" {
		t.Error("keys while the terminal is focused belong to the shell")
	}
	ctrl(m, tea.KeyCtrlB)
	if m.TermFocused || !m.ExplorerFocused || !m.Term.Open {
		t.Fatal("ctrl+b leaves the terminal for the explorer, panel stays open")
	}
	ctrl(m, tea.KeyCtrlJ)
	if !m.TermFocused {
		t.Fatal("ctrl+j on an open, unfocused panel focuses it")
	}
	ctrl(m, tea.KeyCtrlJ)
	if m.Term.Open || m.TermFocused {
		t.Fatal("ctrl+j on a focused panel hides it")
	}
	if m.MainH+3 != m.Height {
		t.Errorf("main pane should take the room back: %d", m.MainH)
	}
}

// Navbar panels are exclusive: opening help, git, find or the terminal
// closes whichever was up.
func TestKeyPanelsAreExclusive(t *testing.T) {
	fakeShell(t)
	m, _ := newTestScreen(t)
	t.Cleanup(m.Term.Close)
	ctrl(m, tea.KeyCtrlJ)
	fkey(m, tea.KeyF5)
	if m.Term.Open || !m.GitOpen {
		t.Fatal("git should replace the terminal")
	}
	ctrl(m, tea.KeyCtrlP)
	if m.GitOpen || !m.Palette.Open {
		t.Fatal("find should replace git")
	}
	fkey(m, tea.KeyF1)
	if m.Palette.Open || !m.ShowHelp {
		t.Fatal("help should replace find")
	}
	ctrl(m, tea.KeyCtrlJ)
	if m.ShowHelp || !m.Term.Open {
		t.Fatal("the terminal should replace help")
	}
}
