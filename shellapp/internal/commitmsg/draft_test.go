package commitmsg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hittable/shellapp/internal/llm"
)

func draftDigest() *Digest {
	return &Digest{
		Branch:     "feat/commit-messages",
		Prefix:     "shellapp/",
		Convention: true,
		Scopes:     []string{"shellapp", "workspace"},
		Files: []Change{{
			Path: "shellapp/ui/components/gitpanel/compose.go", Status: 'A', Added: 420,
			Symbols: []string{"GenChunkMsg", "GenDoneMsg"},
			Hunks:   []string{"@@ -0,0 +1,3 @@\n+type GenChunkMsg struct{ Seq int }"},
		}},
	}
}

// The ladder, one row per outcome. A message is always returned; the error is
// only ever about the model, never about the quality of its output.
func TestDraftLadder(t *testing.T) {
	tests := []struct {
		name       string
		turn       llm.FakeTurn
		wantSource Source
		wantErr    error
		wantHeader string
		wantBullet bool
	}{
		{
			name: "a clean subject and grounded bullets are used as they stand",
			turn: llm.FakeTurn{Text: "feat(gitpanel): stream the commit draft into the editor\n\n" +
				"- Added shellapp/ui/components/gitpanel, which streams a generated commit message into the editor, with GenChunkMsg and GenDoneMsg to carry the tokens."},
			wantSource: SourceModel,
			wantHeader: "feat(gitpanel): stream the commit draft into the editor",
			wantBullet: true,
		},
		{
			name:       "a subject with no bullets keeps the mechanical body",
			turn:       llm.FakeTurn{Text: "feat(gitpanel): stream the commit draft"},
			wantSource: SourceRepaired,
			wantHeader: "feat(gitpanel): stream the commit draft",
		},
		{
			name: "an editorial body is dropped, the subject survives",
			turn: llm.FakeTurn{Text: "feat(gitpanel): stream the commit draft\n\n" +
				"- Added a preview pane that users find intuitive and easy to use."},
			wantSource: SourceRepaired,
			wantHeader: "feat(gitpanel): stream the commit draft",
		},
		{
			// The attribution failure: CloseAI is real, but it is not in the
			// directory this bullet names.
			name: "a bullet attributing a symbol to the wrong directory is dropped",
			turn: llm.FakeTurn{Text: "feat(gitpanel): stream the commit draft\n\n" +
				"- Modified shellapp/ui/components/gitpanel to include new APIs for CloseAI."},
			wantSource: SourceRepaired,
			wantHeader: "feat(gitpanel): stream the commit draft",
		},
		{
			name:       "fenced output is repaired",
			turn:       llm.FakeTurn{Text: "```\nfeat(gitpanel): stream the commit draft\n```"},
			wantSource: SourceRepaired,
			wantHeader: "feat(gitpanel): stream the commit draft",
		},
		{
			name:       "chat preamble is repaired",
			turn:       llm.FakeTurn{Text: "Here is the commit message:\n\nFeat: Stream The Commit Draft."},
			wantSource: SourceRepaired,
			wantHeader: "feat: stream The Commit Draft",
		},
		{
			name:       "empty output falls back to the heuristic",
			turn:       llm.FakeTurn{Text: "   \n  "},
			wantSource: SourceHeuristic,
		},
		{
			name:       "transport failure falls back and reports",
			turn:       llm.FakeTurn{Err: llm.ErrUnreachable},
			wantSource: SourceHeuristic,
			wantErr:    llm.ErrUnreachable,
		},
		{
			name:       "timeout falls back and reports",
			turn:       llm.FakeTurn{Err: llm.ErrTimeout},
			wantSource: SourceHeuristic,
			wantErr:    llm.ErrTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &llm.Fake{Default: tt.turn}
			d := draftDigest()
			m, err := DraftStream(context.Background(), f, d, Options{}, nil)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if m.Source != tt.wantSource {
				t.Errorf("Source = %v, want %v", m.Source, tt.wantSource)
			}
			if tt.wantHeader != "" && m.Header() != tt.wantHeader {
				t.Errorf("header = %q, want %q", m.Header(), tt.wantHeader)
			}
			mechanical := Generate(draftDigest(), Options{}).Body
			if tt.wantBullet {
				if m.Body == mechanical || !strings.HasPrefix(m.Body, "- ") {
					t.Errorf("a grounded body should have survived, got %q", m.Body)
				}
			} else if err == nil && m.Body != mechanical {
				t.Errorf("body = %q, want the mechanical summary %q", m.Body, mechanical)
			}
			if strings.TrimSpace(m.String()) == "" {
				t.Error("Draft returned an empty message; there is no such thing as no draft")
			}
			if err == nil {
				if verr := Validate(m.String(), DefaultRules()); verr != nil {
					t.Errorf("drafted %q: %v", m, verr)
				}
			}
		})
	}
}

// Garbage that cannot be repaired is not an error: the caller's editor has to
// be given something valid rather than left holding the rejected text.
func TestDraftRejectsGarbageWithoutAnError(t *testing.T) {
	f := llm.NewFake("I'm sorry, I can't help with that request.")
	d := draftDigest()
	m, err := DraftStream(context.Background(), f, d, Options{}, nil)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if m.Source != SourceHeuristic {
		t.Errorf("Source = %v, want SourceHeuristic", m.Source)
	}
	if m.String() != Generate(draftDigest(), Options{}).String() {
		t.Errorf("rejected output did not fall back to the heuristic draft: %q", m)
	}
}

func TestDraftStreamDeliversDeltas(t *testing.T) {
	f := llm.NewFakeStream("feat(gitpanel):", " stream the draft", " into the editor")
	var got []string
	m, err := DraftStream(context.Background(), f, draftDigest(), Options{}, func(s string) {
		got = append(got, s)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "feat(gitpanel):" || got[2] != " into the editor" {
		t.Errorf("chunks = %q, want the three deltas verbatim", got)
	}
	// The deltas are the model's whole answer: the subject. The body is
	// written from the diff and was never streamed.
	if strings.Join(got, "") != m.Header() {
		t.Errorf("the assembled deltas %q are not the header %q", strings.Join(got, ""), m.Header())
	}
	if m.Body != Generate(draftDigest(), Options{}).Body {
		t.Errorf("body = %q, want the heuristic's factual summary", m.Body)
	}
}

// esc mid-generation: the partial reaches the caller, and the error says
// "cancelled" rather than "the model failed".
func TestDraftStreamCancelKeepsThePartial(t *testing.T) {
	f := &llm.Fake{Default: llm.FakeTurn{
		Chunks: []string{"feat(gitpanel): stream the draft", "\n\nIt arrives", " token by token."},
		Delay:  40 * time.Millisecond,
	}}
	ctx, cancel := context.WithCancel(context.Background())

	var buf strings.Builder
	m, err := DraftStream(ctx, f, draftDigest(), Options{}, func(s string) {
		buf.WriteString(s)
		if strings.Contains(buf.String(), "It arrives") {
			cancel()
		}
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if !strings.HasPrefix(buf.String(), "feat(gitpanel): stream the draft") {
		t.Errorf("the partial was not delivered: %q", buf.String())
	}
	if strings.TrimSpace(m.String()) == "" {
		t.Error("a cancelled draft still has to return a usable message")
	}
	t.Cleanup(cancel)
}

func TestDraftStreamDeadlineFallsBack(t *testing.T) {
	f := &llm.Fake{Default: llm.FakeTurn{
		Chunks: []string{"feat: too slow"},
		Delay:  200 * time.Millisecond,
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	m, err := DraftStream(ctx, f, draftDigest(), Options{}, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if m.Source != SourceHeuristic {
		t.Errorf("Source = %v, want SourceHeuristic", m.Source)
	}
}

func TestDraftWithoutAClient(t *testing.T) {
	m, err := Draft(context.Background(), nil, draftDigest(), Options{})
	if !errors.Is(err, llm.ErrDisabled) {
		t.Fatalf("err = %v, want llm.ErrDisabled", err)
	}
	if m.Source != SourceHeuristic || strings.TrimSpace(m.String()) == "" {
		t.Errorf("no client must still produce the heuristic draft, got %+v", m)
	}
}

// Draft re-asks once at temperature 0; DraftStream never does, because the
// user is already reading the first answer.
func TestDraftReasksOnceButStreamDoesNot(t *testing.T) {
	script := func() *llm.Fake {
		f := &llm.Fake{Default: llm.FakeTurn{Text: "feat(gitpanel): the corrected message"}}
		f.Turns = []llm.FakeTurn{{Text: "I cannot comply with that."}}
		return f
	}

	f := script()
	m, err := Draft(context.Background(), f, draftDigest(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Calls()) != 2 {
		t.Fatalf("Draft made %d calls, want 2 (one re-ask)", len(f.Calls()))
	}
	if m.Source == SourceHeuristic {
		t.Errorf("the re-ask answer was discarded: %+v", m)
	}
	retry := f.Calls()[1]
	if retry.Temperature != 0 {
		t.Errorf("re-ask Temperature = %v, want 0 (greedy)", retry.Temperature)
	}
	first := f.Calls()[0]
	if n := len(retry.Messages); n != len(first.Messages)+2 {
		t.Fatalf("re-ask carried %d messages, want the original %d plus the rejected answer and the reason", n, len(first.Messages))
	}
	rejected, reason := retry.Messages[len(retry.Messages)-2], retry.Messages[len(retry.Messages)-1]
	if rejected.Role != llm.RoleAssistant || rejected.Content != "I cannot comply with that." {
		t.Errorf("the rejected answer was not fed back: %+v", rejected)
	}
	if reason.Role != llm.RoleUser || !strings.Contains(reason.Content, "rejected") {
		t.Errorf("the re-ask does not say what was wrong: %+v", reason)
	}
	// The first request must not have been mutated by building the second.
	if first.Temperature != draftTemperature || first.Messages[len(first.Messages)-1].Role != llm.RoleUser {
		t.Errorf("the re-ask mutated the original request: %+v", first)
	}

	s := script()
	if _, err := DraftStream(context.Background(), s, draftDigest(), Options{}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if len(s.Calls()) != 1 {
		t.Errorf("DraftStream made %d calls, want 1 (no re-ask while streaming)", len(s.Calls()))
	}
}

func TestClientDrafterSatisfiesThePanelInterface(t *testing.T) {
	var _ Drafter = ClientDrafter{Client: llm.NewFake("feat: ok")}

	dr := ClientDrafter{Client: llm.NewFake("feat(gitpanel): add the compose editor\n\nIt opens on the heuristic draft.")}
	m, err := dr.DraftStream(context.Background(), draftDigest(), Options{}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if m.Type != "feat" || m.Scope != "gitpanel" {
		t.Errorf("parsed message = %+v, want feat(gitpanel)", m)
	}
}

func TestDraftDoesNotProbe(t *testing.T) {
	f := llm.NewFake("feat: add the thing")
	if _, err := Draft(context.Background(), f, draftDigest(), Options{}); err != nil {
		t.Fatal(err)
	}
	if f.Probes() != 0 {
		t.Errorf("Draft probed %d times; the caller decides when to check health", f.Probes())
	}
}

func TestParseMessageRoundTrip(t *testing.T) {
	tests := []struct {
		in    string
		rules Rules
		want  Message
	}{
		{"feat(git): add the picker\n\nbody line", DefaultRules(),
			Message{Type: "feat", Scope: "git", Subject: "add the picker", Body: "body line"}},
		{"feat!: drop v1", DefaultRules(), Message{Type: "feat!", Subject: "drop v1"}},
		{"fix: stop the crash", DefaultRules(), Message{Type: "fix", Subject: "stop the crash"}},
		{"add http: support for redirects", PlainRules(),
			Message{Subject: "add http: support for redirects"}},
	}
	for _, tt := range tests {
		got := parseMessage(tt.in, tt.rules, SourceModel)
		tt.want.Source = SourceModel
		if got != tt.want {
			t.Errorf("parseMessage(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
		if got.String() != tt.in {
			t.Errorf("round trip lost text: %q -> %q", tt.in, got.String())
		}
	}
}

// The exact output a real qwen2.5-coder-3b produced against the first version
// of the prompt: the header is a fragment of an instruction sentence, cut
// mid-word by the grammar's ceiling, and the body is the tail of the user turn
// echoed back verbatim. It must never reach a commit.
func TestDraftRejectsAnEchoedPrompt(t *testing.T) {
	d := draftDigest()
	req := BuildRequest(draftDigest(), Options{Type: "feat"})
	tail := req.Messages[len(req.Messages)-1].Content

	echo := "feat(shellapp): add progress, install and error, improve draft to `im\n\n" +
		req.System + "\n" + tail

	f := llm.NewFake(echo)
	m, err := DraftStream(context.Background(), f, d, Options{Type: "feat"}, nil)
	if err != nil {
		t.Fatalf("err = %v, want nil — an echo is rejected, not reported as a failure", err)
	}
	if m.Source != SourceHeuristic {
		t.Fatalf("Source = %v, want SourceHeuristic; the echo was committed as %q", m.Source, m)
	}
	for _, leak := range []string{"staged files", "improve draft to", "`im", "Starting point"} {
		if strings.Contains(m.String(), leak) {
			t.Errorf("prompt text %q survived into the message:\n%s", leak, m)
		}
	}
}

// Echo detection must not fire on a message that merely mentions a path the
// prompt also mentioned.
func TestEchoesIgnoresIncidentalOverlap(t *testing.T) {
	prompt := BuildRequest(draftDigest(), Options{}).System
	good := "feat(gitpanel): stream the commit draft into the editor\n\n" +
		"The editor opens on the heuristic draft and the model replaces it as\ntokens arrive."
	if Echoes(good, prompt) {
		t.Errorf("a genuine message was flagged as an echo:\n%s", good)
	}
	if Echoes(good, "") {
		t.Error("an empty prompt cannot be echoed")
	}
	// Taken from the prompt itself, so the fixture cannot go stale.
	instruction, _, _ := strings.Cut(prompt, "\n")
	if !Echoes("feat: x\n\n"+instruction, prompt) {
		t.Errorf("a verbatim instruction line was not detected: %q", instruction)
	}
}

// One wrong claim in six used to discard five correct ones, leaving the user
// watching the model's body be replaced by the mechanical file list. Keep what
// is grounded; drop only what is not.
func TestKeepBulletsDropsTheBadBulletNotTheBody(t *testing.T) {
	sp := spec{Blocks: []specBlock{
		{Dir: "internal/llm", New: true, API: []string{"Complete", "Stream"}},
		{Dir: "ui/screens", API: []string{"CloseAI"}},
	}}
	lines := []string{
		"- Added internal/llm, with Complete and Stream to talk to the server.",
		"- Modified ui/screens, adding CloseAI to stop the server.",
		"- Modified internal/llm, adding CloseAI.", // wrong directory
	}
	body, clean := keepBullets(lines, sp)
	if body == "" {
		t.Fatal("whole body discarded for one bad bullet")
	}
	if clean {
		t.Error("a partial keep must report false so Source becomes SourceRepaired")
	}
	if strings.Count(body, "\n") != 1 {
		t.Errorf("want the two good bullets, got:\n%s", body)
	}
	if strings.Contains(body, "Modified internal/llm") {
		t.Error("kept the mis-attributed bullet")
	}
}

// If most of it is wrong it is not a description of the change, and the
// mechanical body is the honest answer.
func TestKeepBulletsGivesUpWhenMostAreUngrounded(t *testing.T) {
	sp := spec{Blocks: []specBlock{{Dir: "internal/llm", New: true, API: []string{"Complete"}}}}
	lines := []string{
		"- Added internal/llm, with Complete.",
		"- Added internal/nowhere, with Invented.",
		"- Added internal/alsofake, with AlsoInvented.",
	}
	if body, _ := keepBullets(lines, sp); body != "" {
		t.Errorf("kept a body that is mostly invented:\n%s", body)
	}
}

// A slash is not always a path: "loading/saving the config" is English.
func TestGroundReadsSlashedWordsAsProse(t *testing.T) {
	sp := spec{Blocks: []specBlock{{Dir: "internal/appconfig", New: true, API: []string{"Load", "Save"}}}}
	ok := "- Added internal/appconfig, with Load and Save for loading/saving the config file(s)."
	if err := ground(ok, sp); err != nil {
		t.Errorf("rejected English containing a slash: %v", err)
	}
	if err := ground("- Added internal/invented, which does things.", sp); err == nil {
		t.Error("accepted a path that is not in the change")
	}
}
