package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// isolate keeps the developer's global / system git config (signing, hooks,
// default branch, aliases) out of the test, so it behaves the same on CI.
func isolate(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "gitconfig")
	os.WriteFile(cfg, []byte("[user]\n\tname = Tester\n\temail = t@x\n[commit]\n\tgpgsign = false\n[init]\n\tdefaultBranch = main\n"), 0o644)
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	return tmp
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// emptyRepo is a fresh `git init` with no commits.
func emptyRepo(t *testing.T) (*Repo, string) {
	t.Helper()
	dir := filepath.Join(isolate(t), "repo")
	os.MkdirAll(dir, 0o755)
	gitIn(t, dir, "init", "-q", "-b", "main")
	r := Open(dir)
	if r == nil {
		t.Fatal("Open returned nil for a fresh repo")
	}
	return r, dir
}

// seededRepo has one commit containing a.txt and b.txt.
func seededRepo(t *testing.T) (*Repo, string) {
	t.Helper()
	r, dir := emptyRepo(t)
	write(t, dir, "a.txt", "one\ntwo\n")
	write(t, dir, "b.txt", "bee\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "first")
	return r, dir
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func find(st *Status, path string) *FileStatus {
	for i := range st.Files {
		if st.Files[i].Path == path {
			return &st.Files[i]
		}
	}
	return nil
}

func mustStatus(t *testing.T, r *Repo) *Status {
	t.Helper()
	st, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestOpenOutsideARepoIsNil(t *testing.T) {
	dir := filepath.Join(isolate(t), "plain")
	os.MkdirAll(dir, 0o755)
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir)) // never find a repo above the temp dir
	if r := Open(dir); r != nil {
		t.Fatalf("Open(non-repo) = %+v, want nil", r)
	}
	if r := Open(filepath.Join(dir, "missing")); r != nil {
		t.Fatalf("Open(missing dir) = %+v, want nil", r)
	}
}

func TestOpenFromSubdirectory(t *testing.T) {
	_, dir := seededRepo(t)
	sub := filepath.Join(dir, "api", "v1")
	os.MkdirAll(sub, 0o755)
	r := Open(sub)
	if r == nil {
		t.Fatal("Open(subdir) = nil")
	}
	if r.Prefix != "api/v1/" {
		t.Errorf("Prefix = %q, want api/v1/", r.Prefix)
	}
	abs := filepath.Join(sub, "x.hit")
	if rel := r.Rel(abs); rel != "api/v1/x.hit" {
		t.Errorf("Rel = %q", rel)
	}
	if got := r.Abs("api/v1/x.hit"); got != abs {
		t.Errorf("Abs = %q, want %q", got, abs)
	}
	// A path outside Dir but inside the repo resolves against Root.
	if got := r.Abs("a.txt"); filepath.Base(got) != "a.txt" {
		t.Errorf("Abs(outside prefix) = %q", got)
	}
	write(t, sub, "x.hit", "{}\n")
	if f := find(mustStatus(t, r), "api/v1/x.hit"); f == nil || !f.Untracked() {
		t.Errorf("untracked file in subdir not reported: %+v", mustStatus(t, r).Files)
	}
}

func TestEmptyRepoHasNoCommits(t *testing.T) {
	r, dir := emptyRepo(t)
	st := mustStatus(t, r)
	if st.Branch != "main" || len(st.Files) != 0 {
		t.Errorf("empty repo status = %+v", st)
	}
	if _, err := r.Log(10, ""); err == nil {
		t.Log("log on an empty repo returned no error (newer git); fine")
	}
	if bs, _ := r.Branches(); len(bs) != 0 {
		t.Errorf("branches in an empty repo = %+v", bs)
	}
	if ss, err := r.Stashes(); err != nil || len(ss) != 0 {
		t.Errorf("stashes = %v %v", ss, err)
	}
	// First commit straight from an empty repo.
	write(t, dir, "first.txt", "hi\n")
	if err := r.Stage("first.txt"); err != nil {
		t.Fatal(err)
	}
	if f := find(mustStatus(t, r), "first.txt"); f == nil || f.Index != 'A' || f.Badge() != "A" {
		t.Fatalf("staged new file = %+v", f)
	}
	if err := r.Commit("root commit"); err != nil {
		t.Fatal(err)
	}
	cs, err := r.Log(10, "")
	if err != nil || len(cs) != 1 || cs[0].Subject != "root commit" || cs[0].Author != "Tester" {
		t.Fatalf("log after first commit = %+v %v", cs, err)
	}
}

func TestStatusKinds(t *testing.T) {
	r, dir := seededRepo(t)
	write(t, dir, "a.txt", "one\ntwo\nthree\n") // modified, unstaged
	os.Remove(filepath.Join(dir, "b.txt"))      // deleted, unstaged
	write(t, dir, "new/deep.txt", "n\n")        // untracked inside a new folder
	st := mustStatus(t, r)
	if f := find(st, "a.txt"); f == nil || !f.Unstaged() || f.Staged() || f.Badge() != "M" {
		t.Errorf("modified = %+v", f)
	}
	if f := find(st, "b.txt"); f == nil || f.Worktree != 'D' || f.Badge() != "D" {
		t.Errorf("deleted = %+v", f)
	}
	if f := find(st, "new/deep.txt"); f == nil || !f.Untracked() || f.Badge() != "U" {
		t.Errorf("untracked (untracked-files=all lists the file, not the dir) = %+v", st.Files)
	}

	// Staged deletion and modification.
	if err := r.Stage("a.txt", "b.txt"); err != nil {
		t.Fatal(err)
	}
	st = mustStatus(t, r)
	if f := find(st, "a.txt"); f == nil || f.Index != 'M' || f.Unstaged() {
		t.Errorf("staged modify = %+v", f)
	}
	if f := find(st, "b.txt"); f == nil || f.Index != 'D' {
		t.Errorf("staged delete = %+v", f)
	}
	// Partially staged: change the file again after staging.
	write(t, dir, "a.txt", "one\ntwo\nthree\nfour\n")
	if f := find(mustStatus(t, r), "a.txt"); f == nil || !f.Staged() || !f.Unstaged() {
		t.Errorf("partially staged = %+v", f)
	}
	if !strings.Contains(r.Diff(FileStatus{Path: "a.txt", Index: 'M', Worktree: 'M'}, true, false), "+three") {
		t.Error("staged diff misses the staged line")
	}
	if d := r.Diff(FileStatus{Path: "a.txt", Index: 'M', Worktree: 'M'}, false, false); !strings.Contains(d, "+four") || strings.Contains(d, "+three") {
		t.Errorf("unstaged diff should only hold the unstaged line:\n%s", d)
	}
	if err := r.Unstage("a.txt"); err != nil {
		t.Fatal(err)
	}
	if f := find(mustStatus(t, r), "a.txt"); f == nil || f.Staged() {
		t.Errorf("after unstage = %+v", f)
	}
}

func TestStatusRename(t *testing.T) {
	r, dir := seededRepo(t)
	gitIn(t, dir, "mv", "a.txt", "renamed.txt")
	st := mustStatus(t, r)
	f := find(st, "renamed.txt")
	if f == nil || f.Index != 'R' || f.Badge() != "R" {
		t.Fatalf("rename = %+v (files %+v)", f, st.Files)
	}
	if find(st, "a.txt") != nil {
		t.Errorf("original path listed as its own entry: %+v", st.Files)
	}
	// The entry after a rename (the origPath) must not swallow the next file.
	write(t, dir, "zz.txt", "z\n")
	if find(mustStatus(t, r), "zz.txt") == nil {
		t.Errorf("file after a rename entry lost: %+v", mustStatus(t, r).Files)
	}
}

// Names git would C-quote without -z: spaces, non-ASCII, quotes, backslashes.
func TestStatusUnusualFileNames(t *testing.T) {
	r, dir := seededRepo(t)
	names := []string{"my file.txt", "café.hit", "日本語.md", `q"uote.txt`, "tab\there.txt"}
	for _, n := range names {
		write(t, dir, n, "x\n")
	}
	st := mustStatus(t, r)
	for _, n := range names {
		f := find(st, n)
		if f == nil {
			t.Fatalf("%q missing from status: %+v", n, st.Files)
		}
		if !strings.Contains(r.Diff(*f, false, false), "+x") {
			t.Errorf("untracked diff for %q empty", n)
		}
		if got := r.Abs(n); got != filepath.Join(dir, n) {
			t.Errorf("Abs(%q) = %q", n, got)
		}
	}
	if err := r.Stage(names...); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if f := find(mustStatus(t, r), n); f == nil || f.Index != 'A' {
			t.Errorf("%q not staged: %+v", n, f)
		}
	}
	if err := r.Commit("odd names"); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "mv", "café.hit", "thé.hit")
	if f := find(mustStatus(t, r), "thé.hit"); f == nil || f.Index != 'R' {
		t.Errorf("unicode rename = %+v", mustStatus(t, r).Files)
	}
	if cs, err := r.Log(5, "日本語.md"); err != nil || len(cs) != 1 {
		t.Errorf("file history for a unicode path = %+v %v", cs, err)
	}
	if bl, err := r.Blame("my file.txt"); err != nil || len(bl) != 1 || bl[0].Summary != "odd names" {
		t.Errorf("blame for a name with a space = %+v %v", bl, err)
	}
}

func TestCommitRefusals(t *testing.T) {
	r, dir := seededRepo(t)
	if err := r.Commit("nothing staged"); err == nil {
		t.Error("commit with nothing staged should fail")
	}
	write(t, dir, "a.txt", "changed\n")
	r.Stage("a.txt")
	if err := r.Commit(""); err == nil {
		t.Error("commit with an empty message should fail")
	}
	if err := r.Commit("subject\n\nbody line one\nbody line two"); err != nil {
		t.Fatal(err)
	}
	show := r.Show("HEAD")
	for _, want := range []string{"subject", "body line two", "a.txt", "+changed"} {
		if !strings.Contains(show, want) {
			t.Errorf("Show missing %q:\n%s", want, show)
		}
	}
	patch := r.ShowPatch("HEAD")
	if !strings.Contains(patch, "diff --git a/a.txt b/a.txt") || strings.Contains(patch, "1 file changed") {
		t.Errorf("ShowPatch should be patch only, no stat:\n%s", patch)
	}
	if r.Show("no-such-rev") != "" {
		t.Error("Show of an unknown rev should be empty, not an error dump")
	}
}

func TestDiscardSingleFile(t *testing.T) {
	r, dir := seededRepo(t)
	write(t, dir, "a.txt", "scratch\n")
	write(t, dir, "junk.txt", "j\n")
	st := mustStatus(t, r)
	if err := r.Discard(*find(st, "a.txt")); err != nil {
		t.Fatal(err)
	}
	if err := r.Discard(*find(st, "junk.txt")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(b) != "one\ntwo\n" {
		t.Errorf("discard did not restore a.txt: %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "junk.txt")); !os.IsNotExist(err) {
		t.Error("discarding an untracked file should delete it")
	}
	if len(mustStatus(t, r).Files) != 0 {
		t.Errorf("tree not clean: %+v", mustStatus(t, r).Files)
	}
}

func TestBranchesCreateSwitchDelete(t *testing.T) {
	r, dir := seededRepo(t)
	if err := r.CreateBranch("feature/login"); err != nil {
		t.Fatal(err)
	}
	if st := mustStatus(t, r); st.Branch != "feature/login" {
		t.Errorf("after create, branch = %q", st.Branch)
	}
	write(t, dir, "login.txt", "l\n")
	r.Stage("login.txt")
	r.Commit("add login")
	bs, err := r.Branches()
	if err != nil || len(bs) != 2 {
		t.Fatalf("branches = %+v %v", bs, err)
	}
	var cur, other Branch
	for _, b := range bs {
		if b.Current {
			cur = b
		} else {
			other = b
		}
	}
	if cur.Name != "feature/login" || cur.Subject != "add login" || other.Name != "main" {
		t.Errorf("current/other = %+v / %+v", cur, other)
	}
	if err := r.Checkout("main"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "login.txt")); !os.IsNotExist(err) {
		t.Error("checkout main should remove the feature branch's file")
	}
	if err := r.CreateBranch("feature/login"); err == nil {
		t.Error("creating an existing branch should fail")
	}
	if err := r.CreateBranch("bad name"); err == nil {
		t.Error("an invalid branch name should fail")
	}
	if err := r.Checkout("does-not-exist"); err == nil {
		t.Error("checkout of a missing branch should fail")
	}
	if err := r.DeleteBranch("feature/login"); err != nil {
		t.Fatal(err)
	}
	if bs, _ := r.Branches(); len(bs) != 1 || bs[0].Name != "main" {
		t.Errorf("after delete = %+v", bs)
	}
}

func TestDetachedHead(t *testing.T) {
	r, dir := seededRepo(t)
	write(t, dir, "a.txt", "second\n")
	r.Stage("a.txt")
	r.Commit("second")
	gitIn(t, dir, "checkout", "-q", "HEAD~1")
	st := mustStatus(t, r)
	if st.Branch != "(detached)" {
		t.Errorf("detached branch = %q", st.Branch)
	}
	if _, err := r.Branches(); err != nil {
		t.Errorf("branches while detached: %v", err)
	}
	if cs, err := r.Log(10, ""); err != nil || len(cs) != 1 {
		t.Errorf("log while detached = %+v %v", cs, err)
	}
}

func TestStashPushListShowPopDrop(t *testing.T) {
	r, dir := seededRepo(t)
	write(t, dir, "a.txt", "wip\n")
	write(t, dir, "untracked.txt", "u\n")
	if err := r.StashPush("half done"); err != nil {
		t.Fatal(err)
	}
	if len(mustStatus(t, r).Files) != 0 {
		t.Fatalf("stash should include untracked files: %+v", mustStatus(t, r).Files)
	}
	ss, err := r.Stashes()
	if err != nil || len(ss) != 1 || ss[0].Ref != "stash@{0}" || !strings.Contains(ss[0].Subject, "half done") {
		t.Fatalf("stashes = %+v %v", ss, err)
	}
	if show := r.StashShow("stash@{0}"); !strings.Contains(show, "+wip") || !strings.Contains(show, "untracked.txt") {
		t.Errorf("stash show:\n%s", show)
	}
	// A second stash without a message.
	write(t, dir, "b.txt", "other\n")
	if err := r.StashPush(""); err != nil {
		t.Fatal(err)
	}
	if ss, _ := r.Stashes(); len(ss) != 2 {
		t.Fatalf("want 2 stashes, got %+v", ss)
	}
	if err := r.StashDrop("stash@{0}"); err != nil {
		t.Fatal(err)
	}
	if err := r.StashPop("stash@{0}"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(b) != "wip\n" {
		t.Errorf("pop did not restore a.txt: %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "untracked.txt")); err != nil {
		t.Error("pop did not restore the untracked file")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "b.txt")); string(b) != "bee\n" {
		t.Errorf("dropped stash leaked into the tree: %q", b)
	}
	if ss, _ := r.Stashes(); len(ss) != 0 {
		t.Errorf("stashes after pop = %+v", ss)
	}
	if err := r.StashPop("stash@{0}"); err == nil {
		t.Error("pop with no stashes should fail")
	}
	if err := r.DiscardAll(); err != nil {
		t.Fatal(err)
	}
	if err := r.StashPush(""); err == nil {
		// Nothing to stash: git exits 0 with "No local changes to save" on some
		// versions, so only check that no stash entry was created.
		if ss, _ := r.Stashes(); len(ss) != 0 {
			t.Errorf("stash with a clean tree created %+v", ss)
		}
	}
}

// Push / pull / fetch / sync against a local bare remote: no network.
func TestRemoteSyncWithLocalBare(t *testing.T) {
	r, dir := seededRepo(t)
	bare := filepath.Join(filepath.Dir(dir), "remote.git")
	gitIn(t, filepath.Dir(dir), "init", "-q", "--bare", "-b", "main", bare)
	gitIn(t, dir, "remote", "add", "origin", bare)

	st := mustStatus(t, r)
	if st.Upstream != "" {
		t.Fatalf("upstream before publish = %q", st.Upstream)
	}
	if _, err := r.Sync(st); err != nil { // no upstream: publishes
		t.Fatalf("publish: %v", err)
	}
	st = mustStatus(t, r)
	if st.Upstream != "origin/main" || st.Ahead != 0 || st.Behind != 0 {
		t.Fatalf("after publish = %+v", st)
	}

	write(t, dir, "a.txt", "local\n")
	r.Stage("a.txt")
	r.Commit("local change")
	if st := mustStatus(t, r); st.Ahead != 1 || st.Behind != 0 {
		t.Errorf("ahead/behind = %d/%d, want 1/0", st.Ahead, st.Behind)
	}
	if _, err := r.Push(); err != nil {
		t.Fatal(err)
	}

	// Someone else pushes; fetch shows us behind, pull fast-forwards.
	other := filepath.Join(filepath.Dir(dir), "other")
	gitIn(t, filepath.Dir(dir), "clone", "-q", bare, other)
	write(t, other, "b.txt", "theirs\n")
	gitIn(t, other, "commit", "-qam", "their change")
	gitIn(t, other, "push", "-q")
	if _, err := r.Fetch(); err != nil {
		t.Fatal(err)
	}
	if st := mustStatus(t, r); st.Behind != 1 {
		t.Errorf("behind after fetch = %d", st.Behind)
	}
	if _, err := r.Pull(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "b.txt")); string(b) != "theirs\n" {
		t.Errorf("pull did not bring b.txt: %q", b)
	}
	if _, err := r.Sync(mustStatus(t, r)); err != nil {
		t.Errorf("sync with nothing to do: %v", err)
	}
}

func TestPushWithoutRemoteFails(t *testing.T) {
	r, _ := seededRepo(t)
	if _, err := r.Push(); err == nil {
		t.Error("push with no remote should report an error")
	}
	if _, err := r.Pull(); err == nil {
		t.Error("pull with no upstream should report an error")
	}
}

func TestAgo(t *testing.T) {
	now := time.Now()
	for _, c := range []struct {
		t    time.Time
		want string
	}{
		{time.Time{}, "now"},
		{now.Add(-10 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-3 * time.Hour), "3h ago"},
		{now.Add(-4 * 24 * time.Hour), "4d ago"},
		{now.Add(-65 * 24 * time.Hour), "2mo ago"},
		{now.Add(-800 * 24 * time.Hour), "2y ago"},
	} {
		if got := Ago(c.t); got != c.want {
			t.Errorf("Ago(%v) = %q, want %q", now.Sub(c.t), got, c.want)
		}
	}
}

func TestConflictParsingEdgeCases(t *testing.T) {
	if cs := ParseConflicts("a\n<<<<<<< HEAD\nours\n=======\ntheirs\n"); len(cs) != 0 {
		t.Errorf("unterminated block should not parse: %+v", cs)
	}
	if cs := ParseConflicts("<<<<<<< HEAD\nours\n"); len(cs) != 0 {
		t.Errorf("block without ======= should not parse: %+v", cs)
	}
	if cs := ParseConflicts(""); len(cs) != 0 {
		t.Error("empty content has no conflicts")
	}
	two := "x\n<<<<<<< HEAD\na\n=======\nb\n>>>>>>> br\ny\n<<<<<<< HEAD\nc\n=======\n>>>>>>> br\n"
	cs := ParseConflicts(two)
	if len(cs) != 2 || cs[0].TheirsLabel != "br" || len(cs[1].Theirs) != 0 {
		t.Fatalf("two blocks = %+v", cs)
	}
	if got := Resolve(two, 5, "ours"); got != two {
		t.Error("out-of-range index must leave content untouched")
	}
	if got := Resolve(two, -1, "ours"); got != two {
		t.Error("negative index must leave content untouched")
	}
	got := Resolve(Resolve(two, 1, "theirs"), 0, "both")
	if got != "x\na\nb\ny\n" {
		t.Errorf("resolve both blocks = %q", got)
	}
}
