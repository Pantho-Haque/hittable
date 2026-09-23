package gitpanel

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/internal/commitmsg"
	"github.com/hittable/shellapp/internal/gitx"
)

// composeRepo builds a repo with conventional history and one staged change,
// so the type picker is exercised rather than skipped.
func composeRepo(t *testing.T) *gitx.Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@x",
			"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@x")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@x")
	run("config", "user.name", "T")

	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Conventional history, so Collect reports Convention and the picker opens.
	for _, c := range []struct{ file, body, msg string }{
		{"a.go", "package a\n", "feat(core): add a"},
		{"b.go", "package b\n", "fix(core): correct b"},
		{"c.go", "package c\n", "feat(core): add c"},
	} {
		write(c.file, c.body)
		run("add", ".")
		run("commit", "-q", "-m", c.msg)
	}
	if err := os.MkdirAll(filepath.Join(dir, "internal/drafter"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("internal/drafter/drafter.go", "// Package drafter turns staged changes into a commit message.\n"+
		"package drafter\n\n"+
		"// Draft writes the message.\nfunc Draft() {}\n\n"+
		"// Cancel stops an in-flight draft.\nfunc Cancel() {}\n")
	run("add", ".")

	return gitx.Open(dir)
}

func newComposePanel(t *testing.T) *Panel {
	t.Helper()
	p := New(composeRepo(t))
	p.SetSize(100, 30)
	p.Refresh()
	return p
}

// fakeDrafter scripts a generation without any HTTP. With Send wired the
// panel runs generation on a goroutine, so written is closed once every chunk
// has reached the buffer: a test that pokes GenChunk before that is asserting
// on a race it will usually, but not always, win.
type fakeDrafter struct {
	chunks  []string
	final   string
	err     error
	written chan struct{}
}

func newFakeDrafter(chunks []string, final string) *fakeDrafter {
	return &fakeDrafter{chunks: chunks, final: final, written: make(chan struct{})}
}

func (f *fakeDrafter) DraftStream(ctx context.Context, d *commitmsg.Digest, opts commitmsg.Options, onChunk func(string)) (commitmsg.Message, error) {
	for _, c := range f.chunks {
		onChunk(c)
	}
	if f.written != nil {
		close(f.written)
	}
	if f.err != nil {
		return commitmsg.Message{}, f.err
	}
	return commitmsg.Message{Subject: f.final, Source: commitmsg.SourceModel}, nil
}

// awaitChunks blocks until the drafter has handed every chunk to the panel.
func awaitChunks(t *testing.T, f *fakeDrafter) {
	t.Helper()
	select {
	case <-f.written:
	case <-time.After(5 * time.Second):
		t.Fatal("drafter never produced its chunks")
	}
}

func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestComposeOpensWithHeuristicDraftAndPicker(t *testing.T) {
	p := newComposePanel(t)
	p.Section = SecStatus
	p.startCompose()

	if p.prompt != promptType {
		t.Fatalf("conventional history should open the type picker, got prompt %v", p.prompt)
	}
	if p.composeType == "" {
		t.Error("picker should preselect an inferred type")
	}
	if foot := p.typePickerFoot(100); !strings.Contains(foot, p.composeType) {
		t.Errorf("picker footer missing the selected type: %q", foot)
	}

	p.handleTypeKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !p.Composing || p.MsgEditor == nil {
		t.Fatal("enter should open the compose editor")
	}
	got := p.MsgEditor.GetContent()
	if !strings.HasPrefix(got, p.composeType) {
		t.Errorf("draft %q should start with the chosen type %q", got, p.composeType)
	}
}

// The picker's footer has to survive the narrow end of the frame-size
// regression: it shares one truncated row with everything else.
func TestTypePickerFitsNarrowFrames(t *testing.T) {
	p := newComposePanel(t)
	p.composeType = "refactor"
	for _, w := range []int{120, 80, 60, 50, 40, 24} {
		if got := ansi.StringWidth(p.typePickerFoot(w)); got > w {
			t.Errorf("picker at width %d rendered %d columns", w, got)
		}
	}
}

func TestComposeStreamsModelOutput(t *testing.T) {
	p := newComposePanel(t)
	f := newFakeDrafter([]string{"feat(core): add ", "the drafter"}, "feat(core): add the drafter")
	p.Drafter = f
	p.Send = func(tea.Msg) {}

	p.startCompose()
	p.handleTypeKey(tea.KeyMsg{Type: tea.KeyEnter})
	awaitChunks(t, f)

	p.GenChunk(GenChunkMsg{Seq: p.genSeq})
	if got := p.MsgEditor.GetContent(); !strings.Contains(got, "add the drafter") {
		t.Errorf("streamed buffer = %q, want the chunks applied", got)
	}
	p.GenDone(GenDoneMsg{Seq: p.genSeq, Text: "feat(core): add the drafter"})
	if p.Generating || p.Busy != "" {
		t.Error("GenDone should clear the busy state")
	}
	if got := p.MsgEditor.GetContent(); got != "feat(core): add the drafter" {
		t.Errorf("final buffer = %q", got)
	}
}

func TestComposeNeverOverwritesTyping(t *testing.T) {
	p := newComposePanel(t)
	f := newFakeDrafter([]string{"model text"}, "model text")
	p.Drafter = f
	p.Send = func(tea.Msg) {}

	p.startCompose()
	p.handleTypeKey(tea.KeyMsg{Type: tea.KeyEnter})

	p.handleComposeKey(tea.KeyMsg{Type: tea.KeyCtrlA})
	p.handleComposeKey(key("mine"))
	typed := p.MsgEditor.GetContent()
	if !strings.Contains(typed, "mine") {
		t.Fatalf("typing did not reach the editor: %q", typed)
	}

	p.GenChunk(GenChunkMsg{Seq: p.genSeq})
	p.GenDone(GenDoneMsg{Seq: p.genSeq, Text: "model text"})
	if got := p.MsgEditor.GetContent(); got != typed {
		t.Errorf("generation overwrote typing: %q, want %q", got, typed)
	}
}

func TestStaleGenerationDropped(t *testing.T) {
	p := newComposePanel(t)
	f := newFakeDrafter([]string{"first"}, "first")
	p.Drafter = f
	p.Send = func(tea.Msg) {}

	p.startCompose()
	p.handleTypeKey(tea.KeyMsg{Type: tea.KeyEnter})
	before := p.MsgEditor.GetContent()

	stale := p.genSeq
	p.genSeq++ // a newer generation has started

	p.GenChunk(GenChunkMsg{Seq: stale})
	p.GenDone(GenDoneMsg{Seq: stale, Text: "stale output"})
	if got := p.MsgEditor.GetContent(); got != before {
		t.Errorf("stale chunk was applied: %q, want %q", got, before)
	}
}

func TestComposeEscCancelsGenerationAndKeepsBuffer(t *testing.T) {
	p := newComposePanel(t)
	f := newFakeDrafter([]string{"partial text"}, "done")
	p.Drafter = f
	p.Send = func(tea.Msg) {}

	p.startCompose()
	p.handleTypeKey(tea.KeyMsg{Type: tea.KeyEnter})
	awaitChunks(t, f)
	p.GenChunk(GenChunkMsg{Seq: p.genSeq})
	partial := p.MsgEditor.GetContent()

	p.handleComposeKey(tea.KeyMsg{Type: tea.KeyEsc})
	if p.Generating {
		t.Error("esc should stop generating")
	}
	if !p.Composing {
		t.Error("esc during generation should stay in the editor, not close it")
	}
	if got := p.MsgEditor.GetContent(); got != partial {
		t.Errorf("cancel lost the buffer: %q, want %q", got, partial)
	}

	// A second esc leaves, and the draft survives for the next c.
	p.handleComposeKey(tea.KeyMsg{Type: tea.KeyEsc})
	if p.Composing {
		t.Error("second esc should close the editor")
	}
	if p.draft != partial {
		t.Errorf("draft = %q, want it kept as %q", p.draft, partial)
	}
}

// With no model installed the flow must be unchanged: the editor still opens,
// still holds a valid message, and nothing waits on anything.
func TestComposeWithoutModel(t *testing.T) {
	p := newComposePanel(t)
	p.Drafter = nil

	p.startCompose()
	p.handleTypeKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !p.Composing {
		t.Fatal("editor should open with no model installed")
	}
	if p.Generating {
		t.Error("nothing should be generating without a drafter")
	}
	if strings.TrimSpace(p.MsgEditor.GetContent()) == "" {
		t.Error("heuristic draft should be non-empty")
	}
}

func TestComposeNothingStaged(t *testing.T) {
	p := newComposePanel(t)
	// Unstage everything.
	if _, err := p.Repo.Run("reset", "-q"); err != nil {
		t.Fatal(err)
	}
	p.startCompose()
	if p.Composing || p.prompt == promptType {
		t.Error("nothing staged should not open the picker or the editor")
	}
	if p.Message != "nothing staged" {
		t.Errorf("Message = %q, want %q", p.Message, "nothing staged")
	}
}

// plainRepo has history that is not conventional, so the picker is skipped.
func plainRepo(t *testing.T) *gitx.Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@x",
			"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@x")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@x")
	run("config", "user.name", "T")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644)
	run("add", ".")
	run("commit", "-q", "-m", "Initial import")
	run("commit", "-q", "--allow-empty", "-m", "Tidy up the build")
	run("commit", "-q", "--allow-empty", "-m", "Make the thing work")
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("two\n"), 0o644)
	run("add", ".")
	return gitx.Open(dir)
}

// A repo that does not use conventional commits must not have one imposed on
// it: no picker, no type in the draft, and no type invented by the repair on
// the way to the commit.
func TestPlainRepoNeverGetsAConventionImposed(t *testing.T) {
	p := New(plainRepo(t))
	p.SetSize(100, 30)
	p.Refresh()

	p.startCompose()
	if p.prompt == promptType {
		t.Error("non-conventional history should not open the type picker")
	}
	if !p.Composing {
		t.Fatal("compose editor should open directly")
	}
	if draft := p.MsgEditor.GetContent(); strings.Contains(draft, ":") {
		t.Errorf("draft %q should carry no conventional type", draft)
	}

	p.handleComposeKey(tea.KeyMsg{Type: tea.KeyCtrlA})
	for _, r := range "add greeting" {
		p.handleComposeKey(key(string(r)))
	}
	p.handleComposeKey(tea.KeyMsg{Type: tea.KeyCtrlS})

	out, err := p.Repo.Run("log", "-1", "--pretty=%s")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out); got != "add greeting" {
		t.Errorf("committed subject = %q, want %q (a type was invented)", got, "add greeting")
	}
}

// Rejected model output comes back with no error (the heuristic text replaces
// it), so Source is the only thing that can tell the user their draft is
// mechanical rather than model-written. Without this the footer silently
// claims nothing and the label becomes worthless.
func TestGenDoneLabelsWhereTheDraftCameFrom(t *testing.T) {
	tests := []struct {
		name   string
		source commitmsg.Source
		want   string
	}{
		{"model output used as-is", commitmsg.SourceModel, ""},
		{"model output repaired", commitmsg.SourceRepaired, "reformatted"},
		{"model output discarded", commitmsg.SourceHeuristic, "rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newComposePanel(t)
			p.startCompose()
			p.handleTypeKey(tea.KeyMsg{Type: tea.KeyEnter})

			p.GenDone(GenDoneMsg{Seq: p.genSeq, Text: "feat(core): something", Source: tt.source})
			if tt.want == "" {
				if p.Message != "" {
					t.Errorf("Message = %q, want no label for un-touched model output", p.Message)
				}
				return
			}
			if !strings.Contains(p.Message, tt.want) {
				t.Errorf("Message = %q, want it to mention %q", p.Message, tt.want)
			}
		})
	}
}
