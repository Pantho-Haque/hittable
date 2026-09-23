package commitmsg

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRunner is the whole of the Runner contract: *gitx.Repo satisfies it the
// same way, structurally, which is what keeps this package free of a git
// import.
type gitRunner struct{ dir string }

func (g gitRunner) Run(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", g.dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Tester", "GIT_AUTHOR_EMAIL=t@x",
		"GIT_COMMITTER_NAME=Tester", "GIT_COMMITTER_EMAIL=t@x")
	out, err := cmd.Output()
	return string(out), err
}

// newRepo mirrors internal/gitx/gitx_test.go's helper: the four GIT_*_NAME /
// GIT_*_EMAIL variables are what make the commits work on a machine with no
// git identity configured.
func newRepo(t *testing.T) (gitRunner, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	g := gitRunner{dir: dir}
	run := func(args ...string) {
		t.Helper()
		if out, err := g.Run(args...); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@x")
	run("config", "user.name", "Tester")
	run("config", "commit.gpgsign", "false")
	return g, dir
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func find(t *testing.T, d *Digest, path string) Change {
	t.Helper()
	for _, c := range d.Files {
		if c.Path == path {
			return c
		}
	}
	t.Fatalf("%s is not in the digest (%d files: %v)", path, len(d.Files), paths(d))
	return Change{}
}

func paths(d *Digest) []string {
	var ps []string
	for _, c := range d.Files {
		ps = append(ps, c.Path)
	}
	return ps
}

func TestCollect(t *testing.T) {
	g, dir := newRepo(t)
	run := func(args ...string) {
		t.Helper()
		if out, err := g.Run(args...); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	write(t, dir, "src/app.go", "package src\n\nfunc main() {}\n")
	write(t, dir, "docs/readme.md", "# docs\n")
	write(t, dir, "old.txt", "line one\nline two\nline three\n")
	run("add", "-A")
	run("commit", "-q", "-m", "feat(app): add the app")
	write(t, dir, "src/app.go", "package src\n\nfunc main() { run() }\n")
	run("add", "-A")
	run("commit", "-q", "-m", "fix(app): call run")

	// The staged change under test: an edit, an addition, a deletion, a
	// rename, a binary and a secret.
	write(t, dir, "src/app.go", "package src\n\nfunc main() { run() }\n\nfunc run() {}\n")
	write(t, dir, "src/new.go", "package src\n\nfunc Exported() string { return \"x\" }\n")
	if err := os.Remove(filepath.Join(dir, "docs", "readme.md")); err != nil {
		t.Fatal(err)
	}
	run("mv", "old.txt", "moved.txt")
	write(t, dir, ".env", "API_TOKEN=hunter2\n")
	write(t, dir, "assets/logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00binary\x00bytes")
	run("add", "-A")

	d, err := Collect(g)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if d.Branch != "main" {
		t.Errorf("Branch = %q, want main", d.Branch)
	}
	if d.Prefix != "" {
		t.Errorf("Prefix = %q, want empty at the repository root", d.Prefix)
	}
	if len(d.Files) != 6 {
		t.Fatalf("got %d staged files, want 6: %v", len(d.Files), paths(d))
	}

	app := find(t, d, "src/app.go")
	if app.Status != 'M' || app.Added != 2 || app.Removed != 0 {
		t.Errorf("src/app.go = %+v, want M +2 -0", app)
	}
	if len(app.Hunks) == 0 || !strings.Contains(strings.Join(app.Hunks, "\n"), "+func run() {}") {
		t.Errorf("src/app.go hunks did not carry the added line: %q", app.Hunks)
	}

	added := find(t, d, "src/new.go")
	if added.Status != 'A' {
		t.Errorf("src/new.go status = %q, want A", added.Status)
	}
	if syms := exportedSymbols(added); len(syms) != 1 || syms[0] != "Exported" {
		t.Errorf("exportedSymbols = %v, want [Exported]", syms)
	}

	if gone := find(t, d, "docs/readme.md"); gone.Status != 'D' || gone.Removed != 1 {
		t.Errorf("docs/readme.md = %+v, want D -1", gone)
	}

	moved := find(t, d, "moved.txt")
	if moved.Status != 'R' || moved.OldPath != "old.txt" {
		t.Errorf("moved.txt = %+v, want R from old.txt", moved)
	}

	logo := find(t, d, "assets/logo.png")
	if !logo.Binary {
		t.Errorf("assets/logo.png = %+v, want Binary", logo)
	}
	if len(logo.Hunks) != 0 {
		t.Errorf("a binary must never be read: %q", logo.Hunks)
	}

	env := find(t, d, ".env")
	if len(env.Hunks) != 0 {
		t.Errorf("a secret must never be read: %q", env.Hunks)
	}
	if strings.Contains(d.Pack(DefaultBudget), "hunter2") {
		t.Error("the secret reached the packed prompt")
	}

	// History: two conventional commits, newest first.
	if !d.Convention {
		t.Error("Convention = false for an all-conventional history")
	}
	if len(d.Scopes) == 0 || d.Scopes[0] != "app" {
		t.Errorf("Scopes = %v, want app first", d.Scopes)
	}

	// The whole point: a valid message falls out of a real staged change.
	m := Generate(d, Options{})
	if err := Validate(m.String(), DefaultRules()); err != nil {
		t.Errorf("generated %q: %v", m, err)
	}
}

// The regression behind "add test compose nothing staged": a newly added file
// arrives as one big hunk, and bounding it to head and tail leaves only the
// import block and the last declaration. Symbols are scanned before the
// trimming, and test functions never name a change.
func TestCollectScansSymbolsBeforeTrimmingHunks(t *testing.T) {
	g, dir := newRepo(t)
	run := func(args ...string) {
		t.Helper()
		if out, err := g.Run(args...); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write(t, dir, "seed.txt", "seed\n")
	run("add", "-A")
	run("commit", "-q", "-m", "feat(gitpanel): seed the repo")

	// A file long enough that its single hunk is trimmed, with the exported
	// declarations well past the head and well before the tail.
	var src strings.Builder
	src.WriteString("package gitpanel\n\nimport \"strings\"\n\n")
	for i := 0; i < 30; i++ {
		src.WriteString("// filler comment line to push the declarations out of the head\n")
	}
	src.WriteString("type GenChunkMsg struct{ Seq int }\n\ntype GenDoneMsg struct{ Seq int }\n\n")
	for i := 0; i < 200; i++ {
		src.WriteString("// trailing filler so the declarations fall outside the tail too\n")
	}
	src.WriteString("var _ = strings.TrimSpace\n")
	write(t, dir, "ui/components/gitpanel/compose.go", src.String())

	var test strings.Builder
	test.WriteString("package gitpanel\n\nimport \"testing\"\n\n")
	test.WriteString("func TestComposeStreamsModelOutput(t *testing.T) {}\n\n")
	for i := 0; i < 50; i++ {
		test.WriteString("// filler\n")
	}
	test.WriteString("func TestComposeNothingStaged(t *testing.T) {}\n")
	write(t, dir, "ui/components/gitpanel/compose_test.go", test.String())
	run("add", "-A")

	d, err := Collect(g)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	compose := find(t, d, "ui/components/gitpanel/compose.go")
	// The test only proves anything if the trimming really did hide the
	// declarations from the hunks.
	if strings.Contains(strings.Join(compose.Hunks, "\n"), "GenDoneMsg") {
		t.Fatal("the hunk was not trimmed past the declarations, so this test proves nothing")
	}
	if len(compose.Symbols) != 2 || compose.Symbols[0] != "GenChunkMsg" || compose.Symbols[1] != "GenDoneMsg" {
		t.Errorf("compose.go symbols = %v, want [GenChunkMsg GenDoneMsg]", compose.Symbols)
	}
	if syms := find(t, d, "ui/components/gitpanel/compose_test.go").Symbols; len(syms) != 0 {
		t.Errorf("test file symbols = %v, want none", syms)
	}

	m := Generate(d, Options{})
	if strings.Contains(m.Subject, "compose nothing staged") {
		t.Errorf("subject was named after a test function: %q", m.Subject)
	}
	if m.Subject != "add gen chunk msg and gen done msg" {
		t.Errorf("subject = %q, want it named after the real declarations", m.Subject)
	}
	if err := Validate(m.String(), DefaultRules()); err != nil {
		t.Errorf("%q: %v", m, err)
	}
}

func TestCollectWithNothingStaged(t *testing.T) {
	g, dir := newRepo(t)
	write(t, dir, "a.txt", "one\n")
	if out, err := g.Run("add", "-A"); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	if out, err := g.Run("commit", "-q", "-m", "chore: first"); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	d, err := Collect(g)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(d.Files) != 0 {
		t.Errorf("got %v staged, want nothing", paths(d))
	}
}

// A repository with no commits makes `git log` fail; Collect must tolerate
// that rather than refuse to describe the first commit.
func TestCollectOnARepoWithNoCommits(t *testing.T) {
	g, dir := newRepo(t)
	write(t, dir, "src/main.go", "package main\n\nfunc main() {}\n")
	if out, err := g.Run("add", "-A"); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	d, err := Collect(g)
	if err != nil {
		t.Fatalf("Collect on an empty repo: %v", err)
	}
	if len(d.Files) != 1 || d.Files[0].Status != 'A' {
		t.Fatalf("Files = %+v, want one added file", d.Files)
	}
	if !d.Convention {
		t.Error("with no history to contradict it the convention should hold")
	}
	if got := Generate(d, Options{}).Header(); got != "feat: add main.go" {
		t.Errorf("header = %q, want %q", got, "feat: add main.go")
	}
}

// Collect must distinguish "nothing staged" from "git failed": Repo.Diff
// swallowing its error is the defect this package does not inherit.
func TestCollectReturnsGitErrors(t *testing.T) {
	if _, err := Collect(failingRunner{}); err == nil {
		t.Fatal("Collect swallowed a git failure")
	}
}

type failingRunner struct{}

func (failingRunner) Run(args ...string) (string, error) {
	if len(args) > 1 && args[0] == "diff" {
		return "", errNotARepo
	}
	return "", nil
}

var errNotARepo = &gitError{"not a git repository"}

type gitError struct{ msg string }

func (e *gitError) Error() string { return e.msg }
