package gitpanel

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"github.com/hittable/shellapp/internal/gitx"
)

// flowRepo is a repo with one commit (a.txt, b.txt), isolated from the
// developer's git config so signing / hooks / default branch can't leak in.
func flowRepo(t *testing.T) (*gitx.Repo, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "gitconfig")
	os.WriteFile(cfg, []byte("[user]\n\tname = Tester\n\temail = t@x\n[commit]\n\tgpgsign = false\n"), 0o644)
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := filepath.Join(tmp, "repo")
	os.MkdirAll(dir, 0o755)
	gitRun(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "a.txt", "one\ntwo\n")
	writeFile(t, dir, "b.txt", "bee\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "first")
	return gitx.Open(dir), dir
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func flowPanel(t *testing.T, r *gitx.Repo) *Panel {
	t.Helper()
	p := New(r)
	p.SetSize(120, 36)
	p.Refresh()
	return p
}

func press(p *Panel, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		case "ctrl+s":
			msg = tea.KeyMsg{Type: tea.KeyCtrlS}
		case "ctrl+a":
			msg = tea.KeyMsg{Type: tea.KeyCtrlA}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		p.HandleKey(msg)
	}
}

func typeText(p *Panel, s string) {
	for _, r := range s {
		if r == ' ' {
			p.HandleKey(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
			continue
		}
		p.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// selectFile puts the cursor on the row for path in the staged or changes group.
func selectFile(t *testing.T, p *Panel, path string, staged bool) {
	t.Helper()
	for i, r := range p.rows {
		if r.file != nil && r.file.Path == path && r.staged == staged {
			p.Cursor = i
			p.loadDetail()
			return
		}
	}
	t.Fatalf("no row for %q (staged=%v); rows: %s", path, staged, rowDump(p))
}

func rowDump(p *Panel) string {
	var b []string
	for _, r := range p.rows {
		switch {
		case r.file != nil:
			b = append(b, r.file.Badge()+" "+r.file.Path)
		case r.branch != nil:
			b = append(b, "branch "+r.branch.Name)
		case r.stash != nil:
			b = append(b, "stash "+r.stash.Ref)
		default:
			b = append(b, r.text)
		}
	}
	return strings.Join(b, " | ")
}

func view(p *Panel) string { return ansi.Strip(p.View(zone.New())) }

func status(t *testing.T, r *gitx.Repo) *gitx.Status {
	t.Helper()
	st, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestPanelStatusShowsEveryKindOfChange(t *testing.T) {
	r, dir := flowRepo(t)
	writeFile(t, dir, "a.txt", "one\ntwo\nthree\n")
	os.Remove(filepath.Join(dir, "b.txt"))
	writeFile(t, dir, "new file.txt", "n\n")
	writeFile(t, dir, "naïve.md", "u\n")
	p := flowPanel(t, r)
	v := view(p)
	for _, want := range []string{"Staged Changes (0)", "Changes (4)", "M a.txt", "D b.txt", "U new file.txt", "U naïve.md", "⎇ main"} {
		if !strings.Contains(v, want) {
			t.Errorf("status view missing %q:\n%s", want, v)
		}
	}
	// The diff of the selected file is shown on the right.
	selectFile(t, p, "a.txt", false)
	if v := view(p); !strings.Contains(v, "+three") {
		t.Errorf("diff pane does not show the change:\n%s", v)
	}
	selectFile(t, p, "naïve.md", false)
	if v := view(p); !strings.Contains(v, "+u") {
		t.Errorf("diff of an untracked unicode file is empty:\n%s", v)
	}
}

func TestPanelStageUnstageKeys(t *testing.T) {
	r, dir := flowRepo(t)
	writeFile(t, dir, "a.txt", "changed\n")
	writeFile(t, dir, "café.txt", "c\n")
	p := flowPanel(t, r)

	selectFile(t, p, "a.txt", false)
	press(p, "s")
	if !strings.Contains(p.Message, "stage a.txt ✓") {
		t.Errorf("message = %q", p.Message)
	}
	selectFile(t, p, "a.txt", true)
	press(p, "u")
	selectFile(t, p, "a.txt", false)
	press(p, "+")
	selectFile(t, p, "a.txt", true)
	press(p, "-")
	selectFile(t, p, "a.txt", false)

	// A unicode name round-trips through stage (git must get the raw path).
	selectFile(t, p, "café.txt", false)
	press(p, "s")
	if !strings.HasSuffix(p.Message, "✓") {
		t.Fatalf("staging a unicode file failed: %q", p.Message)
	}
	selectFile(t, p, "café.txt", true)

	press(p, "a") // stage all
	if v := view(p); !strings.Contains(v, "Staged Changes (2)") || !strings.Contains(v, "Changes (0)") {
		t.Errorf("stage all:\n%s", v)
	}
	press(p, "A") // unstage all
	if v := view(p); !strings.Contains(v, "Staged Changes (0)") || !strings.Contains(v, "Changes (2)") {
		t.Errorf("unstage all:\n%s", v)
	}
}

func TestPanelHeaderEnterCollapsesGroup(t *testing.T) {
	r, dir := flowRepo(t)
	writeFile(t, dir, "a.txt", "changed\n")
	p := flowPanel(t, r)
	for i, row := range p.rows {
		if row.header && strings.Contains(row.text, "Changes") && !strings.Contains(row.text, "Staged") {
			p.Cursor = i
		}
	}
	press(p, "enter")
	if !p.collapsed[1] || strings.Contains(view(p), "M a.txt") {
		t.Fatalf("enter on the Changes header should collapse it:\n%s", view(p))
	}
	press(p, "enter")
	if p.collapsed[1] || !strings.Contains(view(p), "M a.txt") {
		t.Fatalf("second enter should expand it:\n%s", view(p))
	}
}

func TestPanelEnterOpensFile(t *testing.T) {
	r, dir := flowRepo(t)
	writeFile(t, dir, "a.txt", "changed\n")
	p := flowPanel(t, r)
	var opened string
	p.OnOpenFile = func(abs string) { opened = abs }
	selectFile(t, p, "a.txt", false)
	press(p, "enter")
	if opened != filepath.Join(dir, "a.txt") {
		t.Errorf("enter opened %q", opened)
	}
}

func TestPanelDiscardAsksFirst(t *testing.T) {
	r, dir := flowRepo(t)
	writeFile(t, dir, "a.txt", "precious edit\n")
	p := flowPanel(t, r)
	selectFile(t, p, "a.txt", false)

	press(p, "d")
	if !p.PromptOpen() || !strings.Contains(view(p), "discard changes in a.txt (y/n)") {
		t.Fatalf("d should ask for confirmation:\n%s", view(p))
	}
	press(p, "n")
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(b) != "precious edit\n" {
		t.Fatal("n must keep the change")
	}
	press(p, "d", "esc")
	if p.PromptOpen() {
		t.Fatal("esc should dismiss the confirm")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(b) != "precious edit\n" {
		t.Fatal("esc must keep the change")
	}
	// Other keys while confirming do nothing (no accidental discard).
	press(p, "d", "q", "s")
	if !p.PromptOpen() {
		t.Fatal("unrelated keys must not close the confirm")
	}
	press(p, "y")
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(b) != "one\ntwo\n" {
		t.Fatalf("y should discard, got %q", b)
	}
}

func TestPanelUndoAllAndOnChanged(t *testing.T) {
	r, dir := flowRepo(t)
	writeFile(t, dir, "a.txt", "x\n")
	writeFile(t, dir, "junk.txt", "j\n")
	p := flowPanel(t, r)
	changed := 0
	p.OnChanged = func() { changed++ }
	press(p, "D")
	if !strings.Contains(view(p), "undo all changes (y/n)") {
		t.Fatalf("D should confirm:\n%s", view(p))
	}
	press(p, "enter") // enter = yes
	if n := len(status(t, r).Files); n != 0 {
		t.Errorf("undo all left %d files", n)
	}
	if _, err := os.Stat(filepath.Join(dir, "junk.txt")); !os.IsNotExist(err) {
		t.Error("undo all should delete untracked files")
	}
	if changed == 0 {
		t.Error("OnChanged not called after a mutation (explorer badges would go stale)")
	}
}

func TestPanelCommitFlow(t *testing.T) {
	r, dir := flowRepo(t)
	p := flowPanel(t, r)

	press(p, "c")
	if p.Composing || p.Message != "nothing staged" {
		t.Fatalf("c with nothing staged: composing=%v msg=%q", p.Composing, p.Message)
	}

	writeFile(t, dir, "a.txt", "changed\n")
	r.Stage("a.txt")
	p.Refresh()
	press(p, "c")
	if !p.Composing {
		t.Fatalf("c should open the editor (msg %q)", p.Message)
	}
	// Clear the drafted message: an empty message must be refused.
	press(p, "ctrl+a", "backspace", "ctrl+s")
	if p.Message != "empty commit message" || !p.Composing {
		t.Fatalf("empty message: composing=%v msg=%q", p.Composing, p.Message)
	}
	if cs, _ := r.Log(5, ""); len(cs) != 1 {
		t.Fatal("an empty message must not commit")
	}
	typeText(p, "update a")
	press(p, "ctrl+s")
	if p.Composing {
		t.Fatal("ctrl+s should close the editor")
	}
	cs, _ := r.Log(5, "")
	if len(cs) != 2 || cs[0].Subject != "update a" {
		t.Fatalf("log = %+v (msg %q)", cs, p.Message)
	}
	if len(status(t, r).Files) != 0 {
		t.Error("tree should be clean after commit")
	}
}

func TestPanelComposeEscKeepsDraft(t *testing.T) {
	r, dir := flowRepo(t)
	writeFile(t, dir, "a.txt", "changed\n")
	r.Stage("a.txt")
	p := flowPanel(t, r)
	press(p, "c", "ctrl+a", "backspace")
	typeText(p, "my words")
	press(p, "esc")
	if p.Composing {
		t.Fatal("esc should close the editor")
	}
	press(p, "c")
	if got := p.MsgEditor.GetContent(); got != "my words" {
		t.Errorf("draft after esc + c = %q", got)
	}
}

func TestPanelBranchesFlow(t *testing.T) {
	r, _ := flowRepo(t)
	p := flowPanel(t, r)
	press(p, "3")
	if p.Section != SecBranches || !strings.Contains(view(p), "main") {
		t.Fatalf("section 3:\n%s", view(p))
	}

	press(p, "n")
	if !p.PromptOpen() {
		t.Fatal("n should prompt for a branch name")
	}
	typeText(p, "feature-x")
	press(p, "backspace")
	typeText(p, "y")
	press(p, "enter")
	if st := status(t, r); st.Branch != "feature-y" {
		t.Fatalf("branch after create = %q (msg %q)", st.Branch, p.Message)
	}

	// An empty name does nothing.
	press(p, "n", "enter")
	if bs, _ := r.Branches(); len(bs) != 2 {
		t.Errorf("empty name created a branch: %+v", bs)
	}
	// An invalid name reports git's error instead of silently failing.
	press(p, "n")
	typeText(p, "bad..name")
	press(p, "enter")
	if !strings.HasPrefix(p.Message, "branch bad..name: ") {
		t.Errorf("invalid name message = %q", p.Message)
	}

	// d on the current branch is ignored; on another branch it confirms.
	p.Refresh()
	for i, row := range p.rows {
		if row.branch != nil && row.branch.Current {
			p.Cursor = i
		}
	}
	press(p, "d")
	if p.PromptOpen() {
		t.Fatal("deleting the current branch must not be offered")
	}
	for i, row := range p.rows {
		if row.branch != nil && row.branch.Name == "main" {
			p.Cursor = i
		}
	}
	press(p, "enter") // checkout main
	if st := status(t, r); st.Branch != "main" {
		t.Fatalf("enter should check out main, on %q (msg %q)", st.Branch, p.Message)
	}
	for i, row := range p.rows {
		if row.branch != nil && row.branch.Name == "feature-y" {
			p.Cursor = i
		}
	}
	press(p, "d", "y")
	if bs, _ := r.Branches(); len(bs) != 1 {
		t.Errorf("branch not deleted: %+v (msg %q)", bs, p.Message)
	}
}

func TestPanelStashFlow(t *testing.T) {
	r, dir := flowRepo(t)
	writeFile(t, dir, "a.txt", "wip\n")
	p := flowPanel(t, r)

	press(p, "S") // from Status
	typeText(p, "save me")
	press(p, "enter")
	if len(status(t, r).Files) != 0 {
		t.Fatalf("stash from status left changes (msg %q)", p.Message)
	}
	press(p, "4")
	if v := view(p); !strings.Contains(v, "save me") {
		t.Fatalf("stash list:\n%s", v)
	}

	writeFile(t, dir, "b.txt", "second\n")
	press(p, "s", "enter") // stash from the Stashes section, no message
	ss, _ := r.Stashes()
	if len(ss) != 2 {
		t.Fatalf("stashes = %+v", ss)
	}

	p.Refresh()
	p.Cursor = 0 // stash@{0}: the b.txt one
	press(p, "d")
	if !strings.Contains(view(p), "drop stash@{0} (y/n)") {
		t.Fatalf("drop should confirm:\n%s", view(p))
	}
	press(p, "y")
	p.Cursor = 0
	press(p, "enter")
	if !strings.Contains(view(p), "pop stash@{0} (y/n)") {
		t.Fatalf("pop should confirm:\n%s", view(p))
	}
	press(p, "y")
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(b) != "wip\n" {
		t.Errorf("pop restored %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "b.txt")); string(b) != "bee\n" {
		t.Errorf("dropped stash leaked: %q", b)
	}
	if v := view(p); !strings.Contains(v, "No stashes") {
		t.Errorf("empty stash list:\n%s", v)
	}
}

func TestPanelFilter(t *testing.T) {
	r, dir := flowRepo(t)
	writeFile(t, dir, "a.txt", "x\n")
	writeFile(t, dir, "other.md", "y\n")
	p := flowPanel(t, r)
	press(p, "/")
	typeText(p, "OTHER")
	press(p, "enter")
	if v := view(p); strings.Contains(v, "a.txt") || !strings.Contains(v, "other.md") {
		t.Fatalf("filter (case-insensitive) failed:\n%s", v)
	}
	if !p.HandleKey(tea.KeyMsg{Type: tea.KeyEsc}) || p.Filter != "" {
		t.Fatal("esc should clear the filter first, not close the panel")
	}
	if p.HandleKey(tea.KeyMsg{Type: tea.KeyEsc}) {
		t.Fatal("esc with nothing to dismiss should report false (panel closes)")
	}
}

func TestPanelCommitsSection(t *testing.T) {
	r, dir := flowRepo(t)
	writeFile(t, dir, "a.txt", "two\n")
	r.Stage("a.txt")
	r.Commit("touch a")
	writeFile(t, dir, "b.txt", "three\n")
	r.Stage("b.txt")
	r.Commit("touch b")
	p := flowPanel(t, r)
	press(p, "2")
	v := view(p)
	for _, want := range []string{"touch a", "touch b", "first"} {
		if !strings.Contains(v, want) {
			t.Errorf("commits view missing %q:\n%s", want, v)
		}
	}
	// File history: only commits touching the active file.
	p.SetActiveFile(filepath.Join(dir, "a.txt"))
	press(p, "f")
	if !p.FileHistory || len(p.Commits) != 2 {
		t.Fatalf("file history = %+v", p.Commits)
	}
	press(p, "y")
	if !strings.HasPrefix(p.Message, "hash ") {
		t.Errorf("y message = %q", p.Message)
	}
	press(p, "f", "/")
	typeText(p, "touch b")
	press(p, "enter")
	if len(p.rows) != 1 || p.rows[0].commit == nil || p.rows[0].commit.Subject != "touch b" {
		t.Errorf("commit search rows: %s", rowDump(p))
	}
}

func TestPanelSectionNavigationWraps(t *testing.T) {
	r, _ := flowRepo(t)
	p := flowPanel(t, r)
	press(p, "h")
	if p.Section != SecBlame {
		t.Errorf("h from Status = %v, want Blame", p.Section)
	}
	press(p, "l")
	if p.Section != SecStatus {
		t.Errorf("l from Blame = %v, want Status", p.Section)
	}
	for i, want := range []Section{SecStatus, SecCommits, SecBranches, SecStashes, SecBlame} {
		press(p, string(rune('1'+i)))
		if p.Section != want {
			t.Errorf("key %d -> %v", i+1, p.Section)
		}
		_ = view(p)
	}
	if v := view(p); !strings.Contains(v, "Click a file in the explorer") {
		t.Errorf("blame with no active file:\n%s", v)
	}
}

func TestPanelEmptyRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	tmp := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(tmp, "none"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	gitRun(t, tmp, "init", "-q", "-b", "main")
	writeFile(t, tmp, "first.txt", "f\n")
	p := flowPanel(t, gitx.Open(tmp))
	if v := view(p); !strings.Contains(v, "U first.txt") {
		t.Fatalf("empty repo status:\n%s", v)
	}
	// Every section must render on a repo with no commits.
	for _, k := range []string{"2", "3", "4", "5", "1"} {
		press(p, k)
		_ = view(p)
	}
	press(p, "2")
	if !strings.Contains(view(p), "No commits") {
		t.Errorf("commits on an empty repo:\n%s", view(p))
	}
	press(p, "1", "s")
	if f := status(t, p.Repo).Files; len(f) != 1 || f[0].Index != 'A' {
		t.Fatalf("stage in an empty repo: %+v (msg %q)", f, p.Message)
	}
}

func TestPanelDetachedHead(t *testing.T) {
	r, dir := flowRepo(t)
	writeFile(t, dir, "a.txt", "second\n")
	r.Stage("a.txt")
	r.Commit("second")
	gitRun(t, dir, "checkout", "-q", "HEAD~1")
	p := flowPanel(t, r)
	if v := view(p); !strings.Contains(v, "(detached)") {
		t.Errorf("detached HEAD not shown:\n%s", v)
	}
	press(p, "3")
	_ = view(p)
}

func TestPanelWithoutRepo(t *testing.T) {
	p := New(nil)
	p.SetSize(80, 20)
	p.Refresh()
	if v := view(p); !strings.Contains(v, "Not a git repository") {
		t.Fatalf("nil repo view:\n%s", v)
	}
	for _, k := range []string{"s", "c", "1", "3", "n", "enter", "d", "y", "/", "tab", "S"} {
		press(p, k)
	}
	if p.HandleKey(tea.KeyMsg{Type: tea.KeyEsc}) {
		t.Error("esc on a nil repo panel should close it")
	}
	p.SyncAction()
	_ = view(p)
}

func TestPanelTinySizesDoNotPanic(t *testing.T) {
	r, dir := flowRepo(t)
	writeFile(t, dir, "a.txt", "changed\n")
	p := flowPanel(t, r)
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {4, 3}, {10, 5}, {20, 8}, {300, 2}, {2, 200}} {
		for _, k := range []string{"1", "2", "3", "4", "5"} {
			p.SetSize(sz[0], sz[1])
			press(p, k, "down", "up")
			_ = p.View(zone.New())
		}
		p.SetSize(sz[0], sz[1])
		press(p, "1", "tab", "v", "z", "w")
		_ = p.View(zone.New())
		press(p, "esc", "v")
	}
}
