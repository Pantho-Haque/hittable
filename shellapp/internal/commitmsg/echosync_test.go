package commitmsg

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hittable/shellapp/internal/gitx"
)

// promptEchoes is a list of our own prompt's phrases, used to spot an echo
// without having the prompt to hand. It is only as good as its agreement with
// prompt.go: reword the prompt and every fragment silently stops matching
// anything, leaving Validate and Repair waving echoes through while still
// looking like they check.
//
// This ties the two together. If it fails, prompt.go changed and the list
// above it needs the same edit.
func TestPromptEchoFragmentsStillAppearInTheRealPrompt(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(a ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir}, a...)...)
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@x",
			"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@x")
		if o, e := c.CombinedOutput(); e != nil {
			t.Fatalf("git %v: %v %s", a, e, o)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@x")
	run("config", "user.name", "T")
	if err := os.WriteFile(filepath.Join(dir, "seed.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "feat(core): seed the tree")
	run("commit", "-q", "--allow-empty", "-m", "fix(core): stop the panel freezing")
	if err := os.WriteFile(filepath.Join(dir, "next.go"), []byte("package a\n\nfunc Next() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")

	d, err := Collect(gitx.Open(dir))
	if err != nil {
		t.Fatal(err)
	}
	// Some fragments only appear on a conditional branch — a truncated diff, a
	// monorepo sub-path. Union the prompts for every variant, so a fragment
	// that is unreachable in ALL of them is genuinely dead rather than just
	// unexercised by one fixture.
	variants := []*Digest{d}
	trunc := *d
	trunc.Truncated = true
	variants = append(variants, &trunc)
	pre := *d
	pre.Prefix = "shellapp/"
	variants = append(variants, &pre)
	both := *d
	both.Truncated, both.Prefix = true, "shellapp/"
	variants = append(variants, &both)
	plain := *d
	plain.Convention = false
	variants = append(variants, &plain)

	var whole strings.Builder
	for _, v := range variants {
		opts := Options{Type: "feat", Rules: DefaultRules()}
		if !v.Convention {
			opts = Options{Rules: PlainRules()}
		}
		req := BuildRequest(v, opts)
		whole.WriteString(strings.ToLower(req.System))
		for _, m := range req.Messages {
			whole.WriteString("\n")
			whole.WriteString(strings.ToLower(m.Content))
		}
	}
	prompt := whole.String()

	for _, frag := range promptEchoes {
		if !strings.Contains(prompt, frag) {
			t.Errorf("promptEchoes has %q, which no longer appears in the prompt — "+
				"reword it to match prompt.go or drop it, otherwise it guards nothing", frag)
		}
	}
}

// The final turn must end on diff content. Trailing prose is what made a 3B
// continue the instructions instead of answering them, which is the bug that
// reached the user.
func TestFinalTurnEndsOnTheDiffNotOnProse(t *testing.T) {
	d := &Digest{
		Branch:     "main",
		Convention: true,
		Files: []Change{
			{Path: "a.go", Status: 'A', Added: 3, Hunks: []string{"@@ -0,0 +1,3 @@\n+package a\n+\n+func A() {}"}},
		},
	}
	req := BuildRequest(d, Options{Type: "feat", Rules: DefaultRules()})
	if len(req.Messages) == 0 {
		t.Fatal("no messages")
	}
	last := req.Messages[len(req.Messages)-1]
	if last.Role != "user" {
		t.Errorf("last turn is %q, want the user's diff", last.Role)
	}
	for _, cue := range []string{
		"write the commit message", "improve it", "describe only what you can see",
		"the commit type is", "a mechanical draft",
	} {
		if strings.Contains(strings.ToLower(last.Content), cue) {
			t.Errorf("last user turn still ends with instruction prose (%q); "+
				"that is what the model echoed back", cue)
		}
	}
}
