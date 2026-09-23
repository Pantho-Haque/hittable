package gitpanel

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hittable/shellapp/internal/commitmsg"
	"github.com/hittable/shellapp/internal/llm"
	"github.com/hittable/shellapp/internal/llmhost"
)

// TestRealModelThroughThePanel drives the whole path a user drives: press c,
// pick a type, and let a real llama-server stream into the compose editor.
//
// Everything else in this package tests the panel against a scripted drafter,
// which cannot catch a wiring fault — the Send hook being nil in the real app
// shipped exactly that way, and froze the UI for the length of a generation.
// This is the only test that exercises the real supervisor, the real client
// and the real prompt together.
//
//	HITTABLE_AI_E2E=1 go test ./ui/components/gitpanel/ -run TestRealModel -v
func TestRealModelThroughThePanel(t *testing.T) {
	if os.Getenv("HITTABLE_AI_E2E") == "" {
		t.Skip("set HITTABLE_AI_E2E=1 with a model installed")
	}
	if !llmhost.Installed() {
		t.Skip("no model installed")
	}

	sup := llmhost.NewSupervisor()
	defer sup.Close()

	p := newComposePanel(t)
	p.Drafter = commitmsg.ClientDrafter{
		Client: llm.New(llm.Config{Endpoint: sup.Endpoint, Timeout: 4 * time.Minute}),
	}

	// Stand in for the Bubble Tea program: generation runs on a goroutine and
	// posts messages, exactly as it does in the app.
	msgs := make(chan tea.Msg, 256)
	p.Send = func(m tea.Msg) { msgs <- m }

	p.startCompose()
	if p.prompt != promptType {
		t.Fatalf("expected the type picker, got prompt %v", p.prompt)
	}
	p.handleTypeKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !p.Composing {
		t.Fatal("editor did not open")
	}

	// The editor must be usable immediately, before the model answers.
	opening := p.MsgEditor.GetContent()
	if strings.TrimSpace(opening) == "" {
		t.Error("editor opened empty; the draft should be there from the first frame")
	}
	if !p.Generating {
		t.Error("generation did not start")
	}
	t.Logf("opened instantly with:\n%s", opening)

	deadline := time.After(4 * time.Minute)
	var chunks int
	for done := false; !done; {
		select {
		case m := <-msgs:
			switch v := m.(type) {
			case GenChunkMsg:
				chunks++
				p.GenChunk(v)
			case GenDoneMsg:
				if v.Err != nil {
					t.Fatalf("generation failed: %v", v.Err)
				}
				p.GenDone(v)
				t.Logf("source=%s chunks=%d", v.Source, chunks)
				done = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for the model")
		}
	}

	final := p.MsgEditor.GetContent()
	t.Logf("\n--- in the editor ---\n%s\n---------------------", final)

	if p.Generating || p.Busy != "" {
		t.Error("busy state not cleared")
	}
	if final == opening {
		t.Error("the model never replaced the draft")
	}
	if err := commitmsg.Validate(final, p.composeRules); err != nil {
		t.Errorf("what is in the editor would not pass commitlint: %v", err)
	}
	// The body is model-written but grounded against the spec, so the package
	// that was actually staged has to appear in it. A body that names nothing
	// real is the failure this whole path exists to prevent.
	if !strings.Contains(final, "drafter") {
		t.Errorf("body does not mention the staged package:\n%s", final)
	}
}
