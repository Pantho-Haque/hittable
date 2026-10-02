package gitx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Line-level stage / revert: the file gains a line at the top and one at the
// bottom; each is handled on its own, VS Code style.
func TestSubPatchStageAndRevertLines(t *testing.T) {
	r := newRepo(t)
	path := filepath.Join(r.Root, "a.txt")
	os.WriteFile(path, []byte("zero\none\ntwo\nthree\n"), 0o644)
	f := FileStatus{Path: "a.txt", Worktree: 'M'}
	diff := r.Diff(f, false, false)
	lines := strings.Split(diff, "\n")
	at := func(prefix string) int {
		for i, l := range lines {
			if l == prefix {
				return i
			}
		}
		t.Fatalf("no %q in\n%s", prefix, diff)
		return -1
	}
	zero, three := at("+zero"), at("+three")

	// Stage only "zero".
	patch, ok := SubPatch(diff, zero, zero, false)
	if !ok || strings.Contains(patch, "+three") || !strings.Contains(patch, "+zero") {
		t.Fatalf("stage patch wrong (ok=%v):\n%s", ok, patch)
	}
	if err := r.Apply(patch, true, false); err != nil {
		t.Fatalf("apply --cached: %v\n%s", err, patch)
	}
	staged := r.Diff(f, true, false)
	if !strings.Contains(staged, "+zero") || strings.Contains(staged, "+three") {
		t.Fatalf("index after staging one line:\n%s", staged)
	}

	// Revert only "three" in the working tree.
	diff = r.Diff(f, false, false)
	lines = strings.Split(diff, "\n")
	three = at("+three")
	patch, ok = SubPatch(diff, three, three, true)
	if !ok {
		t.Fatal("nothing selected")
	}
	if err := r.Apply(patch, false, true); err != nil {
		t.Fatalf("apply --reverse: %v\n%s", err, patch)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "zero\none\ntwo\n" {
		t.Fatalf("working tree after revert: %q", b)
	}

	// Unstage "zero" again: index goes back to HEAD.
	staged = r.Diff(f, true, false)
	lines = strings.Split(staged, "\n")
	zero = at("+zero")
	patch, _ = SubPatch(staged, zero, zero, true)
	if err := r.Apply(patch, true, true); err != nil {
		t.Fatalf("apply --cached --reverse: %v\n%s", err, patch)
	}
	if s := r.Diff(f, true, false); strings.TrimSpace(s) != "" {
		t.Fatalf("index not clean:\n%s", s)
	}

	// A range with no change in it yields no patch.
	if _, ok := SubPatch(diff, 0, 0, false); ok {
		t.Error("header-only range should select nothing")
	}
}

func TestGraph(t *testing.T) {
	r := newRepo(t)
	g, err := r.Graph("main", 10)
	if err != nil || len(g) != 1 || !strings.HasPrefix(g[0], "* ") || !strings.Contains(g[0], "first commit") {
		t.Fatalf("graph = %q, %v", g, err)
	}
}
