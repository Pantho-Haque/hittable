package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/internal/hitfile"
	"github.com/hittable/shellapp/ui/components/explorer"
)

// cursorTo puts the explorer cursor on the node at path, expanding parents.
func cursorTo(t *testing.T, m *MainScreen, path string) {
	t.Helper()
	m.setExplorerFocused(true)
	m.Focus = FocusExplorerPane
	for i := 0; i < 10; i++ {
		for idx, n := range m.Explorer.Visible {
			if n.Path == path {
				m.Explorer.Cursor = idx
				return
			}
			if n.Kind == explorer.NodeDir && !n.Expanded && strings.HasPrefix(path, n.Path+string(os.PathSeparator)) {
				m.Explorer.Toggle(n)
				break
			}
		}
	}
	t.Fatalf("%s not in the tree", path)
}

// prompt types a name into the open explorer prompt and confirms it,
// delivering the resulting message like the program would.
func prompt(m *MainScreen, s string) {
	if s != "" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		m.Update(cmd())
	}
}

func clearPrompt(m *MainScreen) {
	for m.Explorer.PromptValue != "" {
		m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// ctrl+n in a folder creates a request with the default shape and opens it
// in Runner; a nested name creates the folders; a taken name is refused.
func TestExplorerNewFile(t *testing.T) {
	m, hd := newTestScreen(t)
	cursorTo(t, m, hd)
	ctrl(m, tea.KeyCtrlN)
	if m.Explorer.PromptKind != explorer.PromptNewFile {
		t.Fatal("ctrl+n should open the new-file prompt")
	}
	prompt(m, "users.hit")
	p := filepath.Join(hd, "users.hit")
	h := readHit(t, p)
	if h["method"] != "GET" || h["response"] != nil {
		t.Errorf("new .hit shape: %v", h)
	}
	if hd, _ := h["headers"].(map[string]any); hd["Content-Type"] != "application/json" {
		t.Errorf("new .hit headers: %v", h["headers"])
	}
	if m.ActiveFile != p || m.ViewMode != ViewRunner {
		t.Errorf("new request should open in Runner: %s view=%d", m.ActiveFile, m.ViewMode)
	}
	if c := m.Explorer.Visible[m.Explorer.Cursor].Path; c != p {
		t.Errorf("cursor should land on the new file, on %s", c)
	}

	cursorTo(t, m, hd)
	ctrl(m, tea.KeyCtrlN)
	prompt(m, "docs/deep/readme.md")
	nested := filepath.Join(hd, "docs", "deep", "readme.md")
	if !exists(nested) || m.ActiveFile != nested || !m.isMarkdown() {
		t.Fatalf("nested create: exists=%v active=%s", exists(nested), m.ActiveFile)
	}

	cursorTo(t, m, hd)
	ctrl(m, tea.KeyCtrlN)
	prompt(m, "users.hit")
	if !strings.Contains(m.StatusBar, "Already exists") {
		t.Errorf("duplicate name: status %q", m.StatusBar)
	}

	// Empty names and esc do nothing.
	before, _ := os.ReadDir(hd)
	cursorTo(t, m, hd)
	ctrl(m, tea.KeyCtrlN)
	prompt(m, "   ")
	ctrl(m, tea.KeyCtrlN)
	runes(m, "nope.txt")
	ctrl(m, tea.KeyEsc)
	if m.Explorer.PromptKind != explorer.PromptNone {
		t.Error("esc should close the prompt")
	}
	if after, _ := os.ReadDir(hd); len(after) != len(before) || exists(filepath.Join(hd, "nope.txt")) {
		t.Error("an empty or cancelled prompt created something")
	}
}

// While a prompt is open, keys are text: ctrl+ shortcuts, ? and g must not
// fire, so typing a name never sends a request or opens a panel.
func TestExplorerPromptSwallowsShortcuts(t *testing.T) {
	m, hd := newTestScreen(t)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	cursorTo(t, m, hd)
	ctrl(m, tea.KeyCtrlN)
	runes(m, "g?")
	ctrl(m, tea.KeyCtrlT)
	ctrl(m, tea.KeyCtrlR)
	if m.GitOpen || m.ShowHelp || m.ViewMode != ViewRunner || m.Sending {
		t.Fatalf("a shortcut fired inside the prompt: git=%v help=%v view=%d", m.GitOpen, m.ShowHelp, m.ViewMode)
	}
	if m.Explorer.PromptValue != "g?" {
		t.Errorf("prompt value %q", m.Explorer.PromptValue)
	}
}

// ctrl+f in the explorer makes a folder (not a file).
func TestExplorerNewFolder(t *testing.T) {
	m, hd := newTestScreen(t)
	cursorTo(t, m, filepath.Join(hd, "a.hit")) // on a file: the folder goes next to it
	ctrl(m, tea.KeyCtrlF)
	if m.Explorer.PromptKind != explorer.PromptNewFolder {
		t.Fatal("ctrl+f in the explorer is New Folder")
	}
	prompt(m, "auth")
	if fi, err := os.Stat(filepath.Join(hd, "auth")); err != nil || !fi.IsDir() {
		t.Fatalf("auth/ not created: %v", err)
	}
	if m.ActiveFile != "" {
		t.Error("a new folder opens nothing")
	}
}

// Renaming the open file keeps it open under the new name; later edits go
// to the new file and the old one never comes back.
func TestExplorerRenameOpenFile(t *testing.T) {
	m, hd := newTestScreen(t)
	old := filepath.Join(hd, "a.hit")
	m.openFileRaw(old)
	m.Body.SetContent("unsaved edit")
	cursorTo(t, m, old)
	ctrl(m, tea.KeyCtrlE)
	if m.Explorer.PromptValue != "a.hit" {
		t.Fatalf("rename prompt should start with the name, got %q", m.Explorer.PromptValue)
	}
	clearPrompt(m)
	prompt(m, "login.hit")
	nw := filepath.Join(hd, "login.hit")
	if exists(old) || !exists(nw) || m.ActiveFile != nw {
		t.Fatalf("rename: old=%v new=%v active=%s", exists(old), exists(nw), m.ActiveFile)
	}
	if readHit(t, nw)["body"] != "unsaved edit" {
		t.Error("the edit made before renaming was lost")
	}
	m.Body.SetContent("after rename")
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow()
	if exists(old) {
		t.Error("a save after rename resurrected the old file")
	}
	if readHit(t, nw)["body"] != "after rename" {
		t.Error("edits after rename went missing")
	}
}

// Renaming the folder that holds the open file moves it with the folder; the
// next save must land in the new place, not recreate the old folder.
func TestExplorerRenameFolderOfOpenFile(t *testing.T) {
	m, hd := newTestScreen(t)
	dir := filepath.Join(hd, "users")
	os.MkdirAll(dir, 0o755)
	f := filepath.Join(dir, "list.hit")
	writeHit(t, f, hitfileGET("http://x/users"))
	m.Explorer.RebuildTree()
	m.openFileRaw(f)
	cursorTo(t, m, dir)
	ctrl(m, tea.KeyCtrlE)
	clearPrompt(m)
	prompt(m, "people")
	moved := filepath.Join(hd, "people", "list.hit")
	if !exists(moved) || exists(dir) {
		t.Fatalf("folder rename: moved=%v old=%v", exists(moved), exists(dir))
	}
	m.Body.SetContent("edited after the move")
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow()
	if exists(dir) {
		t.Fatal("saving the open file recreated the renamed folder")
	}
	if readHit(t, moved)["body"] != "edited after the move" {
		t.Errorf("edit did not reach the moved file: %v", readHit(t, moved)["body"])
	}
	if m.ActiveFile != moved {
		t.Errorf("active file = %s, want %s", m.ActiveFile, moved)
	}
}

// Rename onto an existing name is refused and nothing changes; renaming into
// an existing folder ("sub/name") moves the file there.
func TestExplorerRenameConflictsAndMove(t *testing.T) {
	m, hd := newTestScreen(t)
	a := filepath.Join(hd, "a.hit")
	md := filepath.Join(hd, "notes.md")
	cursorTo(t, m, a)
	ctrl(m, tea.KeyCtrlE)
	clearPrompt(m)
	prompt(m, "notes.md")
	if !strings.Contains(m.StatusBar, "exists") {
		t.Errorf("status %q", m.StatusBar)
	}
	if b, _ := os.ReadFile(md); string(b) != "hello\n" || !exists(a) {
		t.Fatal("a refused rename changed files")
	}

	os.MkdirAll(filepath.Join(hd, "sub"), 0o755)
	m.Explorer.RebuildTree()
	cursorTo(t, m, a)
	ctrl(m, tea.KeyCtrlE)
	clearPrompt(m)
	prompt(m, filepath.Join("sub", "a.hit"))
	if !exists(filepath.Join(hd, "sub", "a.hit")) || exists(a) {
		t.Error("rename with a folder prefix should move the file")
	}
}

// ctrl+d asks first: n keeps the file, y deletes it. Deleting the open file
// closes it, and its pending autosave must not bring it back.
func TestExplorerDelete(t *testing.T) {
	m, hd := newTestScreen(t)
	a := filepath.Join(hd, "a.hit")
	cursorTo(t, m, a)
	ctrl(m, tea.KeyCtrlD)
	if !m.Explorer.DeletePending {
		t.Fatal("ctrl+d should ask for confirmation")
	}
	runes(m, "n")
	if m.Explorer.DeletePending || !exists(a) {
		t.Fatal("n should cancel the delete")
	}

	m.openFileRaw(a)
	m.Body.SetContent("pending write")
	m.saveAndEnqueue() // queued, not flushed
	cursorTo(t, m, a)
	ctrl(m, tea.KeyCtrlD)
	_, cmd := runes(m, "y")
	if cmd == nil {
		t.Fatal("y should confirm")
	}
	m.Update(cmd())
	if exists(a) {
		t.Fatal("y did not delete")
	}
	if m.ActiveFile != "" {
		t.Error("deleting the open file should close it")
	}
	m.WriteQueue.FlushNow()
	if exists(a) {
		t.Error("a queued autosave resurrected the deleted file")
	}

	// Deleting a folder that holds the open file closes it too.
	dir := filepath.Join(hd, "grp")
	os.MkdirAll(dir, 0o755)
	inner := filepath.Join(dir, "x.md")
	os.WriteFile(inner, []byte("x"), 0o644)
	m.Explorer.RebuildTree()
	m.openFileRaw(inner)
	cursorTo(t, m, dir)
	ctrl(m, tea.KeyCtrlD)
	_, cmd = runes(m, "y")
	m.Update(cmd())
	if exists(dir) || m.ActiveFile != "" {
		t.Errorf("folder delete: exists=%v active=%q", exists(dir), m.ActiveFile)
	}
}

// x and right-click open the context menu; it navigates with j/k and runs
// the item on enter; esc closes it.
func TestExplorerContextMenu(t *testing.T) {
	m, hd := newTestScreen(t)
	a := filepath.Join(hd, "a.hit")
	cursorTo(t, m, a)
	runes(m, "x")
	if !m.Explorer.MenuOpen {
		t.Fatal("x should open the menu")
	}
	ctrl(m, tea.KeyEsc)
	if m.Explorer.MenuOpen || m.ActiveFile != "" {
		t.Fatal("esc should only close the menu")
	}
	runes(m, "x")
	runes(m, "j")
	runes(m, "j") // New File, New Folder, Rename
	ctrl(m, tea.KeyEnter)
	if m.Explorer.PromptKind != explorer.PromptRename {
		t.Fatalf("third item should be Rename, prompt=%d", m.Explorer.PromptKind)
	}
	ctrl(m, tea.KeyEsc)

	// Right-click on a row opens the menu on that row.
	draw(m)
	x, y := cellOf(t, m, "notes.md")
	m.Update(tea.MouseMsg{Type: tea.MouseRight, Button: tea.MouseButtonRight, Action: tea.MouseActionPress, X: x, Y: y})
	if !m.Explorer.MenuOpen || m.Explorer.Visible[m.Explorer.Cursor].Name != "notes.md" {
		t.Fatalf("right-click: open=%v cursor on %s", m.Explorer.MenuOpen, m.Explorer.Visible[m.Explorer.Cursor].Name)
	}
	runes(m, "j")
	runes(m, "j")
	runes(m, "j") // Delete
	ctrl(m, tea.KeyEnter)
	if !m.Explorer.DeletePending {
		t.Fatal("fourth item should be Delete")
	}
	runes(m, "n")
	if !exists(filepath.Join(hd, "notes.md")) {
		t.Error("cancelled delete removed the file")
	}
}

// Keyboard navigation in the tree: j/k/↑/↓ move, enter/l expand, h collapses
// then goes to the parent, end/home jump, r picks up files made outside.
func TestExplorerKeyboardNavigation(t *testing.T) {
	m, hd := newTestScreen(t)
	sub := filepath.Join(hd, "sub")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "z.md"), []byte("z"), 0o644)
	m.Explorer.RebuildTree()
	m.setExplorerFocused(true)
	cur := func() string { return m.Explorer.Visible[m.Explorer.Cursor].Path }

	cursorTo(t, m, hd)
	if m.Explorer.Visible[m.Explorer.Cursor].Expanded {
		t.Fatal("hittable/ starts collapsed")
	}
	ctrl(m, tea.KeyEnter)
	if !m.Explorer.Visible[m.Explorer.Cursor].Expanded {
		t.Fatal("enter on a folder should expand it")
	}
	ctrl(m, tea.KeyEnter)
	if m.Explorer.Visible[m.Explorer.Cursor].Expanded {
		t.Fatal("enter on an open folder should collapse it")
	}
	runes(m, "l")
	if !m.Explorer.Visible[m.Explorer.Cursor].Expanded {
		t.Fatal("l should expand")
	}
	if m.ActiveFile != "" || !m.ExplorerFocused {
		t.Error("expanding a folder keeps focus in the explorer")
	}
	runes(m, "j")
	first := cur()
	ctrl(m, tea.KeyDown)
	runes(m, "k")
	if cur() != first {
		t.Errorf("j then ↓ then k should be one row down, at %s", cur())
	}
	ctrl(m, tea.KeyUp)
	if cur() != hd {
		t.Errorf("↑ should return to hittable/, at %s", cur())
	}
	cursorTo(t, m, filepath.Join(sub, "z.md"))
	runes(m, "h")
	if cur() != sub {
		t.Errorf("h on a file goes to its folder, at %s", cur())
	}
	runes(m, "h")
	if m.Explorer.Visible[m.Explorer.Cursor].Expanded {
		t.Error("h on an open folder collapses it")
	}
	ctrl(m, tea.KeyEnd)
	if m.Explorer.Cursor != len(m.Explorer.Visible)-1 {
		t.Error("end should go to the last row")
	}
	runes(m, "G")
	if m.Explorer.Cursor != len(m.Explorer.Visible)-1 {
		t.Error("G should go to the last row")
	}
	ctrl(m, tea.KeyHome)
	if m.Explorer.Cursor != 0 {
		t.Error("home should go to the top")
	}
	ctrl(m, tea.KeyRight)
	if !m.ExplorerFocused {
		t.Error("→ never leaves the sidebar")
	}

	// A file created outside the app shows up after r.
	os.WriteFile(filepath.Join(hd, "external.hit"), []byte("{}"), 0o644)
	runes(m, "r")
	found := false
	for _, n := range m.Explorer.Visible {
		found = found || n.Name == "external.hit"
	}
	if !found {
		t.Error("r should refresh the tree")
	}

	// enter on a file opens it and moves focus to the main pane.
	cursorTo(t, m, filepath.Join(hd, "notes.md"))
	ctrl(m, tea.KeyEnter)
	if m.ActiveFile != filepath.Join(hd, "notes.md") || m.ExplorerFocused {
		t.Errorf("enter on a file: active=%s explorerFocused=%v", m.ActiveFile, m.ExplorerFocused)
	}
}

func hitfileGET(url string) hitfile.HitFile { return hitfile.HitFile{Method: "GET", URL: url} }
