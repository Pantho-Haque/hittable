package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func newRepo(t *testing.T) *Repo {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Tester", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=Tester", "GIT_COMMITTER_EMAIL=t@x")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@x")
	run("config", "user.name", "Tester")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\ntwo\n"), 0o644)
	run("add", ".")
	run("commit", "-q", "-m", "first commit")
	r := Open(filepath.Join(dir))
	if r == nil {
		t.Fatal("Open returned nil")
	}
	return r
}

func TestStatusStageCommitLogBlame(t *testing.T) {
	r := newRepo(t)
	os.WriteFile(filepath.Join(r.Root, "a.txt"), []byte("one\ntwo\nthree\n"), 0o644)
	os.WriteFile(filepath.Join(r.Root, "new.txt"), []byte("x\n"), 0o644)

	st, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "main" || len(st.Files) != 2 {
		t.Fatalf("status: %+v", st)
	}
	var mod, unt FileStatus
	for _, f := range st.Files {
		if f.Path == "a.txt" {
			mod = f
		} else {
			unt = f
		}
	}
	if !mod.Unstaged() || mod.Badge() != "M" || !unt.Untracked() || unt.Badge() != "U" {
		t.Errorf("badges: %+v %+v", mod, unt)
	}
	if d := r.Diff(mod, false, false); !contains(d, "+three") {
		t.Errorf("diff: %s", d)
	}
	if d := r.Diff(unt, false, false); !contains(d, "+x") {
		t.Errorf("untracked diff: %s", d)
	}

	if err := r.Stage("a.txt"); err != nil {
		t.Fatal(err)
	}
	st, _ = r.Status()
	for _, f := range st.Files {
		if f.Path == "a.txt" && !f.Staged() {
			t.Errorf("a.txt should be staged: %+v", f)
		}
	}
	if err := r.Commit("second commit"); err != nil {
		t.Fatal(err)
	}
	log, err := r.Log(10, "")
	if err != nil || len(log) != 2 || log[0].Subject != "second commit" {
		t.Fatalf("log: %v %+v", err, log)
	}
	if show := r.Show(log[0].Hash); !contains(show, "+three") {
		t.Errorf("show: %s", show)
	}

	bl, err := r.Blame("a.txt")
	if err != nil || len(bl) != 3 {
		t.Fatalf("blame: %v %+v", err, bl)
	}
	if bl[0].Summary != "first commit" || bl[2].Summary != "second commit" || bl[0].Author != "Tester" {
		t.Errorf("blame lines: %+v", bl)
	}

	if err := r.CreateBranch("feature"); err != nil {
		t.Fatal(err)
	}
	bs, _ := r.Branches()
	var cur string
	for _, b := range bs {
		if b.Current {
			cur = b.Name
		}
	}
	if cur != "feature" || len(bs) != 2 {
		t.Errorf("branches: %+v", bs)
	}
	if err := r.Checkout("main"); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteBranch("feature"); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(r.Root, "a.txt"), []byte("changed\n"), 0o644)
	if err := r.StashPush("wip"); err != nil {
		t.Fatal(err)
	}
	ss, _ := r.Stashes()
	if len(ss) != 1 || !contains(ss[0].Subject, "wip") {
		t.Fatalf("stashes: %+v", ss)
	}
	if show := r.StashShow(ss[0].Ref); !contains(show, "+changed") {
		t.Errorf("stash show: %s", show)
	}
	if err := r.StashPop(ss[0].Ref); err != nil {
		t.Fatal(err)
	}
	st, _ = r.Status()
	if len(st.Files) != 2 { // a.txt modified + new.txt untracked
		t.Errorf("after pop: %+v", st.Files)
	}
}

func contains(s, sub string) bool { return len(sub) == 0 || (len(s) > 0 && indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestMergeConflictFlow(t *testing.T) {
	r := newRepo(t)
	env := []string{"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@x"}
	must := func(args ...string) {
		if out, err := r.RunEnv(env, args...); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	must("checkout", "-q", "-b", "feature")
	os.WriteFile(filepath.Join(r.Root, "a.txt"), []byte("theirs\ntwo\n"), 0o644)
	must("commit", "-q", "-am", "feature change")
	must("checkout", "-q", "main")
	os.WriteFile(filepath.Join(r.Root, "a.txt"), []byte("ours\ntwo\n"), 0o644)
	must("commit", "-q", "-am", "main change")
	if _, err := r.RunEnv(env, "merge", "feature"); err == nil {
		t.Fatal("merge should conflict")
	}
	ok, kind := r.MergeInProgress()
	if !ok || kind != "merge" {
		t.Fatalf("merge in progress: %v %q", ok, kind)
	}
	st, _ := r.Status()
	if len(st.Files) != 1 || !st.Files[0].Conflict() || st.Files[0].Badge() != "!" {
		t.Fatalf("conflict status: %+v", st.Files)
	}
	content, _ := os.ReadFile(filepath.Join(r.Root, "a.txt"))
	cs := ParseConflicts(string(content))
	if len(cs) != 1 || cs[0].Ours[0] != "ours" || cs[0].Theirs[0] != "theirs" {
		t.Fatalf("parse: %+v", cs)
	}
	resolved := Resolve(string(content), 0, "both")
	if resolved != "ours\ntheirs\ntwo\n" {
		t.Fatalf("resolve both: %q", resolved)
	}
	os.WriteFile(filepath.Join(r.Root, "a.txt"), []byte(resolved), 0o644)
	if err := r.Stage("a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := r.MergeContinue("merge"); err != nil {
		t.Fatalf("continue: %v", err)
	}
	if ok, _ := r.MergeInProgress(); ok {
		t.Error("merge should be finished")
	}
	log, _ := r.Log(5, "")
	if len(log) < 4 || !contains(log[0].Subject, "Merge") {
		t.Errorf("log after merge: %+v", log)
	}
}

func TestDiscardAll(t *testing.T) {
	r := newRepo(t)
	os.WriteFile(filepath.Join(r.Root, "a.txt"), []byte("clobbered\n"), 0o644)
	os.WriteFile(filepath.Join(r.Root, "new.txt"), []byte("x\n"), 0o644)

	if err := r.DiscardAll(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(r.Root, "a.txt")); string(b) != "one\ntwo\n" {
		t.Fatalf("tracked file not reverted: %q", b)
	}
	if _, err := os.Stat(filepath.Join(r.Root, "new.txt")); !os.IsNotExist(err) {
		t.Fatal("untracked file not removed")
	}
	st, err := r.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Files) != 0 {
		t.Fatalf("expected clean tree, got %v", st.Files)
	}
}
