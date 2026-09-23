package commitmsg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hittable/shellapp/internal/gitx"
	"github.com/hittable/shellapp/internal/llm"
	"github.com/hittable/shellapp/internal/llmhost"
)

// TestRealModelGeneration drives the real installed model end to end. It is
// the only test that can prove the prompt produces a commit message rather
// than an echo of itself, because that failure lives in the model's response
// to a real chat template, not in anything a fake can reproduce.
//
//	HITTABLE_AI_E2E=1 go test ./internal/commitmsg/ -run TestRealModel -v
func TestRealModelGeneration(t *testing.T) {
	if os.Getenv("HITTABLE_AI_E2E") == "" {
		t.Skip("set HITTABLE_AI_E2E=1 with a model installed")
	}
	if !llmhost.Installed() {
		t.Skip("no model installed")
	}
	dir := t.TempDir()
	run := func(a ...string) {
		c := exec.Command("git", append([]string{"-C", dir}, a...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@x")
		if o, e := c.CombinedOutput(); e != nil {
			t.Fatalf("git %v: %v %s", a, e, o)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@x")
	run("config", "user.name", "T")
	os.WriteFile(filepath.Join(dir, "seed.go"), []byte("package a\n"), 0o644)
	run("add", ".")
	run("commit", "-q", "-m", "feat(shellapp): seed the tree")
	run("commit", "-q", "--allow-empty", "-m", "fix(shellapp): stop the git panel freezing during a push")
	run("commit", "-q", "--allow-empty", "-m", "feat(texteditor): fold blocks from indentation")

	// Stage the real compose.go from this change.
	os.MkdirAll(filepath.Join(dir, "ui/components/gitpanel"), 0o755)
	b, err := os.ReadFile("../../ui/components/gitpanel/compose.go")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "ui/components/gitpanel/compose.go"), b, 0o644)
	run("add", ".")

	d, err := Collect(gitx.Open(dir))
	if err != nil {
		t.Fatal(err)
	}

	sup := llmhost.NewSupervisor()
	defer sup.Close()
	c := llm.New(llm.Config{Endpoint: sup.Endpoint, Timeout: 4 * time.Minute})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	start := time.Now()
	var chunks int
	var raw strings.Builder
	msg, err := DraftStream(ctx, c, d, Options{Type: InferType(d), Rules: DefaultRules()}, func(s string) { chunks++; raw.WriteString(s) })
	if err != nil {
		t.Fatalf("draft failed: %v", err)
	}
	t.Logf("elapsed=%s chunks=%d source=%s", time.Since(start).Round(time.Second), chunks, msg.Source)
	t.Logf("\n--- RAW MODEL OUTPUT ---\n%s\n--- AFTER FILTER ---\n%s\n---", raw.String(), msg.String())

	if msg.Source == SourceHeuristic {
		t.Errorf("fell back to the heuristic — the model output was rejected")
	}
	req := BuildRequest(d, Options{Type: InferType(d), Rules: DefaultRules()})
	var prompt strings.Builder
	prompt.WriteString(req.System)
	for _, m := range req.Messages {
		prompt.WriteString("\n" + m.Content)
	}
	if Echoes(msg.String(), prompt.String()) {
		t.Error("STILL ECHOING THE PROMPT")
	}
	if err := Validate(msg.String(), DefaultRules()); err != nil {
		t.Errorf("generated message fails commitlint: %v", err)
	}

	// The model is asked for one line and the body is written from the diff.
	// Anything the model said past the first line must not have reached the
	// message, and the body must be the heuristic's, character for character.
	if strings.Contains(msg.Subject, "\n") {
		t.Errorf("the subject spans more than one line: %q", msg.Subject)
	}
	for _, line := range strings.Split(msg.Body, "\n") {
		if line != "" && !strings.HasPrefix(line, "- ") {
			t.Errorf("body line %q is not a bullet", line)
		}
	}
	// Every claim in the body has to be checkable against the specification
	// the model was given — that is the difference between a phrase filter and
	// a truth check.
	if err := ground(msg.Body, buildSpec(d)); err != nil {
		t.Errorf("the committed body is not grounded: %v", err)
	}
	if hasEditorial(msg.Body) {
		t.Errorf("editorial prose reached the body:\n%s", msg.Body)
	}
}
