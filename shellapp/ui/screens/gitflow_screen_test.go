package screens

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
)

// gitScreen builds root/hittable/{a.hit,notes.md,café.hit}, optionally as a
// committed git repo, isolated from the developer's git config.
func gfScreen(t *testing.T, repo bool) (*MainScreen, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "gitconfig")
	os.WriteFile(cfg, []byte("[user]\n\tname = T\n\temail = t@x\n[commit]\n\tgpgsign = false\n"), 0o644)
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := filepath.Join(tmp, "proj")
	t.Setenv("GIT_CEILING_DIRECTORIES", tmp) // never pick up a repo above the temp dir
	hd := filepath.Join(root, "hittable")
	os.MkdirAll(hd, 0o755)
	os.WriteFile(filepath.Join(hd, "env.json"), []byte(`{}`), 0o644)
	os.WriteFile(filepath.Join(hd, "a.hit"), []byte(`{"method":"GET","url":"http://x/a","headers":{},"params":{},"body":"","response":null}`), 0o644)
	os.WriteFile(filepath.Join(hd, "notes.md"), []byte("hello\n"), 0o644)
	os.WriteFile(filepath.Join(hd, "café.hit"), []byte(`{"method":"GET","url":"http://x/c","headers":{},"params":{},"body":"","response":null}`), 0o644)
	if repo {
		for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"commit", "-q", "-m", "init"}} {
			if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v %s", args, err, out)
			}
		}
	}
	m := NewMainScreen(root, zone.New())
	m.SetSize(160, 40)
	return m, hd
}

func gfPlain(m *MainScreen) string { return ansi.Strip(m.View()) }

func gfRunes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestGitPanelOutsideARepo(t *testing.T) {
	m, _ := gfScreen(t, false)
	if m.Repo != nil {
		t.Fatalf("temp dir detected as a repo: %+v", m.Repo)
	}
	if v := gfPlain(m); !strings.Contains(v, "not a git repo") {
		t.Errorf("top bar should say not a git repo:\n%s", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyF5})
	if !m.GitOpen {
		t.Fatal("F5 should still open the panel")
	}
	if v := gfPlain(m); !strings.Contains(v, "Not a git repository") {
		t.Errorf("panel should explain there is no repo:\n%s", v)
	}
	for _, k := range []string{"s", "c", "3", "n", "d", "y", "p"} { // must not panic
		m.Update(gfRunes(k))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.GitOpen {
		t.Error("esc should close the panel")
	}
	m.pollGit() // the 2s poll must be a no-op without a repo
}

func TestGitPanelOpenBindings(t *testing.T) {
	m, _ := gfScreen(t, true)
	for _, c := range []struct {
		name string
		key  tea.KeyMsg
	}{
		{"F5", tea.KeyMsg{Type: tea.KeyF5}},
		{"alt+g", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g"), Alt: true}},
		{"ctrl+g", tea.KeyMsg{Type: tea.KeyCtrlG}},
		{"g in explorer", gfRunes("g")},
	} {
		m.setExplorerFocused(true)
		m.Update(c.key)
		if !m.GitOpen {
			t.Errorf("%s did not open the git panel", c.name)
			continue
		}
		m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
		if m.GitOpen {
			t.Errorf("ctrl+g inside the panel should close it (opened with %s)", c.name)
		}
	}
	// ctrl+b leaves the panel for the explorer.
	m.Update(tea.KeyMsg{Type: tea.KeyF5})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if m.GitOpen || !m.ExplorerFocused {
		t.Errorf("ctrl+b: git=%v explorer=%v", m.GitOpen, m.ExplorerFocused)
	}
}

// A file created outside the app (integrated terminal, another editor) shows
// up in the tree, with its badge, on the next poll.
func TestGitPollPicksUpExternalChanges(t *testing.T) {
	m, hd := gfScreen(t, true)
	m.Explorer.Toggle(m.Explorer.Visible[0]) // expand hittable/
	os.WriteFile(filepath.Join(hd, "fresh.hit"), []byte(`{}`), 0o644)
	os.WriteFile(filepath.Join(hd, "notes.md"), []byte("edited\n"), 0o644)
	m.Update(tea.KeyMsg{Type: tea.KeyF5})

	st, err := m.Repo.Status()
	if err != nil {
		t.Fatal(err)
	}
	m.applyStatus(gitStatusMsg{st: st, key: statusKey(st)})
	if b := m.Explorer.GitStatus[filepath.Join(hd, "fresh.hit")]; b != "U" {
		t.Errorf("fresh.hit badge = %q, want U (badges %v)", b, m.Explorer.GitStatus)
	}
	if b := m.Explorer.GitStatus[filepath.Join(hd, "notes.md")]; b != "M" {
		t.Errorf("notes.md badge = %q, want M", b)
	}
	v := gfPlain(m)
	if !strings.Contains(v, "fresh.hit") || !strings.Contains(v, "Changes (2)") {
		t.Errorf("tree / open panel not refreshed:\n%s", v)
	}
	// An unchanged poll is ignored (no rebuild on every tick).
	before := m.gitKey
	m.applyStatus(gitStatusMsg{st: st, key: statusKey(st)})
	if m.gitKey != before || m.gitPolling {
		t.Error("identical poll should be a no-op")
	}
	m.applyStatus(gitStatusMsg{}) // a failed poll must not clear state
	if len(m.Explorer.GitStatus) == 0 {
		t.Error("failed poll wiped the badges")
	}
}

// Badges for non-ASCII names land on the explorer's path (git would C-quote
// "café.hit" without -z, and the badge would key a path that doesn't exist).
func TestGitBadgeForUnicodeName(t *testing.T) {
	m, hd := gfScreen(t, true)
	p := filepath.Join(hd, "café.hit")
	os.WriteFile(p, []byte(`{"method":"POST"}`), 0o644)
	m.refreshGit(true)
	if b := m.Explorer.GitStatus[p]; b != "M" {
		t.Fatalf("café.hit badge = %q (badges %v)", b, m.Explorer.GitStatus)
	}
	// And the panel can stage it and open it.
	m.Update(tea.KeyMsg{Type: tea.KeyF5})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(gfRunes("s"))
	if st, _ := m.Repo.Status(); len(st.Files) != 1 || st.Files[0].Index != 'M' {
		t.Fatalf("stage via panel: %+v (msg %q)", st.Files, m.Git.Message)
	}
	m.Update(gfRunes("1")) // section reset puts the cursor on the first file: the staged one
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.GitOpen || m.ActiveFile != p {
		t.Errorf("enter should open the file: git=%v active=%q", m.GitOpen, m.ActiveFile)
	}
}

// Discarding a file in the panel must reach the open editor, and the next
// autosave must not write the stale buffer back over the discard.
func TestGitDiscardReloadsOpenEditor(t *testing.T) {
	m, hd := gfScreen(t, true)
	p := filepath.Join(hd, "notes.md")
	m.openFileRaw(p)
	m.TextEd.SetContent(p, "scratch edit\n")
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow()
	if b, _ := os.ReadFile(p); string(b) != "scratch edit\n" {
		t.Fatalf("edit not on disk: %q", b)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyF5})
	m.Update(gfRunes("d"))
	m.Update(gfRunes("y"))
	if b, _ := os.ReadFile(p); string(b) != "hello\n" {
		t.Fatalf("discard did not restore the file: %q (msg %q)", b, m.Git.Message)
	}
	if got := m.Store.Get(p).GetContent(); got != "hello\n" {
		t.Errorf("document still holds the discarded edit: %q", got)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow()
	if b, _ := os.ReadFile(p); string(b) != "hello\n" {
		t.Errorf("autosave resurrected the discarded edit: %q", b)
	}
}

// Opening a sub-directory of a repo: paths are relative to the repo root in
// git but must map back onto the explorer's absolute paths.
func TestGitRootIsSubdirectoryOfRepo(t *testing.T) {
	m, hd := gfScreen(t, true)
	root := filepath.Dir(hd)
	sub := filepath.Join(root, "service")
	os.MkdirAll(filepath.Join(sub, "hittable"), 0o755)
	f := filepath.Join(sub, "hittable", "x.hit")
	os.WriteFile(f, []byte(`{}`), 0o644)

	m = NewMainScreen(sub, m.Zones)
	m.SetSize(160, 40)
	if m.Repo == nil || m.Repo.Prefix != "service/" {
		t.Fatalf("repo = %+v", m.Repo)
	}
	if b := m.Explorer.GitStatus[f]; b != "U" {
		t.Errorf("badge in a sub-root = %q (badges %v)", b, m.Explorer.GitStatus)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyF5})
	if v := gfPlain(m); !strings.Contains(v, "service/hittable/x.hit") {
		t.Errorf("panel should list the repo-relative path:\n%s", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.ActiveFile != f {
		t.Errorf("enter opened %q, want %q", m.ActiveFile, f)
	}
}
