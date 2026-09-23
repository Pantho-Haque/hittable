package commitmsg

import (
	"strings"
	"testing"

	"github.com/hittable/shellapp/internal/llm"
)

func TestBuildRequest(t *testing.T) {
	d := draftDigest()
	req := BuildRequest(d, Options{Type: "feat"})

	if n := len(req.Messages); n%2 != 1 || n < 3 {
		t.Fatalf("got %d messages, want alternating example turns and one final user turn", n)
	}
	for i, m := range req.Messages[:len(req.Messages)-1] {
		want := llm.RoleUser
		if i%2 == 1 {
			want = llm.RoleAssistant
		}
		if m.Role != want {
			t.Errorf("message %d has role %q, want %q", i, m.Role, want)
		}
	}
	last := req.Messages[len(req.Messages)-1]
	if last.Role != llm.RoleUser {
		t.Errorf("the final turn is %q, want a user turn for the model to answer", last.Role)
	}
	user := last.Content
	sys := req.System

	// The grammar is the strongest of the three mechanisms, and narrowing it
	// to the picked type is the whole point of having a picker.
	if req.Grammar == "" {
		t.Error("Grammar is empty; the format would only be discouraged, not enforced")
	}
	if !strings.Contains(req.Grammar, `type    ::= "feat"`) {
		t.Errorf("Grammar was not narrowed to the picked type:\n%s", req.Grammar)
	}
	if !strings.Contains(sys, "Use the type: feat") {
		t.Errorf("the system prompt does not state the picked type:\n%s", sys)
	}

	// The anchor is the floor: a 3B improves a candidate far more reliably
	// than it invents one. It is a constraint, so it lives in the system
	// message — trailing the diff it read as prose to carry on with.
	anchor := Generate(draftDigest(), Options{Type: "feat"}).Subject
	if !strings.Contains(sys, anchor) {
		t.Errorf("the heuristic subject %q is not anchored in the system prompt:\n%s", anchor, sys)
	}

	// The packed diff has to be in there, or there is nothing to describe.
	if !strings.Contains(user, "shellapp/ui/components/gitpanel/compose.go") {
		t.Errorf("the packed digest is missing from the prompt:\n%s", user)
	}

	if req.Temperature != draftTemperature || req.MaxTokens != draftMaxTokens {
		t.Errorf("sampling = %v/%d, want %v/%d", req.Temperature, req.MaxTokens, draftTemperature, draftMaxTokens)
	}
	// The body is bullets now, so a bare newline can no longer end the answer.
	if len(req.Stop) == 0 || req.Stop[0] != "\n\n\n" {
		t.Errorf("Stop = %q, want the blank-run and fence stops", req.Stop)
	}
	if !strings.Contains(req.Grammar, "bullet") {
		t.Errorf("the grammar does not enforce the bullet shape:\n%s", req.Grammar)
	}
	if req.Prompt != "" {
		t.Error("BuildRequest must use the chat route, not /completions")
	}
}

// The bug this whole shape exists to prevent: instructions trailing the user
// turn get continued rather than answered.
func TestBuildRequestPutsNoInstructionsInTheFinalUserTurn(t *testing.T) {
	for _, opts := range []Options{{}, {Type: "feat"}, {Rules: PlainRules()}} {
		req := BuildRequest(draftDigest(), opts)
		user := req.Messages[len(req.Messages)-1].Content

		for _, cue := range []string{
			"Write the commit message", "Improve it if", "A mechanical draft",
			"The commit type is", "do not guess", "Describe only",
		} {
			if strings.Contains(user, cue) {
				t.Errorf("instruction %q leaked into the final user turn:\n%s", cue, user)
			}
		}
		if got := strings.TrimSpace(user); !strings.Contains(got, "(new)") && !strings.Contains(got, "(modified)") {
			t.Errorf("the final user turn should be the specification and nothing else:\n%s", user)
		}
	}
}

// The example is one pair showing the exact shape asked for: a specification
// in, a subject and bullets out.
func TestBuildRequestOneShot(t *testing.T) {
	req := BuildRequest(draftDigest(), Options{})
	if len(req.Messages) != 3 {
		t.Fatalf("got %d messages, want the example pair and the question", len(req.Messages))
	}
	if req.Messages[0].Role != llm.RoleUser || req.Messages[1].Role != llm.RoleAssistant {
		t.Fatalf("the example is not a user/assistant pair: %+v", req.Messages[:2])
	}
	if !strings.Contains(req.Messages[0].Content, "purpose:") || !strings.Contains(req.Messages[0].Content, "new API:") {
		t.Errorf("the example question is not a specification:\n%s", req.Messages[0].Content)
	}
	answer := req.Messages[1].Content
	if err := Validate(answer, DefaultRules()); err != nil {
		t.Errorf("the example answer is not a valid message: %v\n%s", err, answer)
	}
	header, body, _ := strings.Cut(answer, "\n\n")
	if !strings.Contains(header, ":") {
		t.Errorf("the example answer has no conventional header: %q", header)
	}
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "- ") {
			t.Errorf("the example answer is not all bullets: %q", line)
		}
	}
	// It must teach the target shape, not its own content.
	if strings.Contains(req.Messages[2].Content, "httpcache") {
		t.Error("the example leaked into the real question")
	}
}

// A plain repository gets an example without a type, or the model learns to
// write one anyway.
func TestBuildRequestOneShotFollowsTheRules(t *testing.T) {
	req := BuildRequest(draftDigest(), Options{Rules: PlainRules()})
	answer := req.Messages[1].Content
	if err := Validate(answer, PlainRules()); err != nil {
		t.Errorf("plain example is invalid: %v\n%s", err, answer)
	}
	header, _, _ := strings.Cut(answer, "\n")
	if strings.Contains(header, "): ") {
		t.Errorf("a plain prompt got a typed example: %q", header)
	}
}

// The whole prompt has to fit the context window the supervisor asks for.
func TestBuildRequestStaysWithinBudget(t *testing.T) {
	var files []Change
	for i := 0; i < 500; i++ {
		files = append(files, Change{
			Path:   "shellapp/internal/pkg/file" + strings.Repeat("x", i%20) + string(rune('a'+i%26)) + ".go",
			Status: 'M', Added: 40, Removed: 9,
			Hunks: []string{"@@ -1,3 +1,6 @@\n+added line that is reasonably long to pad this out\n context"},
		})
	}
	d := &Digest{Branch: "main", Prefix: "shellapp/", Convention: true, Files: files}
	req := BuildRequest(d, Options{})

	total := len(req.System)
	for _, m := range req.Messages {
		total += len(m.Content)
	}
	if total > DefaultBudget {
		t.Errorf("prompt is %d bytes, over the %d budget", total, DefaultBudget)
	}
	// A change this broad is described by the specification alone; handing
	// over half a diff is what produced "major refactoring and new features".
	if strings.Contains(req.Messages[2].Content, "@@") {
		t.Errorf("a truncated diff reached a large change's prompt:\n%s", req.Messages[2].Content)
	}
}

func TestBuildRequestOnAnUntruncatedDigestSaysNothingAboutTruncation(t *testing.T) {
	req := BuildRequest(draftDigest(), Options{})
	if strings.Contains(req.System, "shortened") {
		t.Error("a complete diff must not be described as shortened")
	}
}
