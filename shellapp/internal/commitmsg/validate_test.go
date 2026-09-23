package commitmsg

import (
	"strings"
	"testing"
)

// One row per commitlint rule, then the failures a 3B model actually produces.
// wantRule is the prefix of the error Validate must return ("" means the
// message is already valid); wantRepaired is the exact text Repair must
// produce with "feat" as the picker's type.
func TestValidateAndRepair(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		wantRule     string
		wantRepaired string
		wantOK       bool
	}{
		{
			name:         "valid header and body",
			in:           "feat(git): add commit message generator\n\n- body line",
			wantRepaired: "feat(git): add commit message generator\n\n- body line",
			wantOK:       true,
		},
		{
			name:         "valid without a body",
			in:           "chore: bump deps",
			wantRepaired: "chore: bump deps",
			wantOK:       true,
		},
		{
			name:         "valid breaking change",
			in:           "feat!: drop v1 of the api",
			wantRepaired: "feat!: drop v1 of the api",
			wantOK:       true,
		},
		{
			name:         "type-enum: invented type",
			in:           "wip: do some things",
			wantRule:     "type-enum",
			wantRepaired: "feat: do some things",
			wantOK:       true,
		},
		{
			name:         "type-enum: synonym is mapped, not replaced",
			in:           "feature(git): add the picker",
			wantRule:     "type-enum",
			wantRepaired: "feat(git): add the picker",
			wantOK:       true,
		},
		{
			name:         "type-case: capitalised type",
			in:           "Feat: add fold all",
			wantRule:     "type-case",
			wantRepaired: "feat: add fold all",
			wantOK:       true,
		},
		{
			name:         "type-empty: no type at all",
			in:           "add fold all",
			wantRule:     "type-empty",
			wantRepaired: "feat: add fold all",
			wantOK:       true,
		},
		{
			name:         "scope-empty: empty parentheses",
			in:           "feat(): add fold all",
			wantRule:     "scope-empty",
			wantRepaired: "feat: add fold all",
			wantOK:       true,
		},
		{
			name:     "subject-empty: nothing after the colon",
			in:       "feat: ",
			wantRule: "subject-empty",
			wantOK:   false,
		},
		{
			name:     "subject-empty: nothing at all",
			in:       "",
			wantRule: "subject-empty",
			wantOK:   false,
		},
		{
			name:         "subject-full-stop",
			in:           "feat: add fold all.",
			wantRule:     "subject-full-stop",
			wantRepaired: "feat: add fold all",
			wantOK:       true,
		},
		{
			name:         "subject-case: sentence case",
			in:           "feat: Add fold all",
			wantRule:     "subject-case",
			wantRepaired: "feat: add fold all",
			wantOK:       true,
		},
		{
			name:         "subject-case: upper case is lowered whole",
			in:           "fix: FIX THE CRASH ON QUIT",
			wantRule:     "subject-case",
			wantRepaired: "fix: fix the crash on quit",
			wantOK:       true,
		},
		{
			// The single most important row: commitlint rejects sentence,
			// start, pascal and upper case as a class, so only the first
			// character may be lowered. Blanket-lowercasing destroys
			// identifiers, and this message is already legal.
			name:         "subject-case: an identifier survives repair unchanged",
			in:           "feat: add FoldAll to the editor",
			wantRepaired: "feat: add FoldAll to the editor",
			wantOK:       true,
		},
		{
			name:         "body-leading-blank: two-line subject",
			in:           "feat: add fold all\nit collapses every block",
			wantRule:     "body-leading-blank",
			wantRepaired: "feat: add fold all\n\nit collapses every block",
			wantOK:       true,
		},
		{
			name:         "fenced model output",
			in:           "```\nfeat: add fold all\n\nit collapses every block\n```",
			wantRule:     "type-empty",
			wantRepaired: "feat: add fold all\n\nit collapses every block",
			wantOK:       true,
		},
		{
			name:         "fenced model output with a language tag",
			in:           "```text\nfeat(git): add the type picker\n```",
			wantRule:     "type-empty",
			wantRepaired: "feat(git): add the type picker",
			wantOK:       true,
		},
		{
			name:         "chat preamble on its own line",
			in:           "Here is the commit message:\n\nfeat: add fold all",
			wantRule:     "type-empty",
			wantRepaired: "feat: add fold all",
			wantOK:       true,
		},
		{
			name:         "chat preamble on the same line",
			in:           "Commit message: feat: add fold all",
			wantRule:     "type-empty",
			wantRepaired: "feat: add fold all",
			wantOK:       true,
		},
		{
			name:         "everything wrong at once",
			in:           "Feat: Add Fold All.",
			wantRule:     "type-case",
			wantRepaired: "feat: add Fold All",
			wantOK:       true,
		},
		{
			// A refusal is not a commit message with formatting problems.
			// Repairing it would commit "chore: i'm sorry, i can't help".
			name:     "a model refusal is not repairable",
			in:       "I'm sorry, I can't help with that request.",
			wantRule: "type-empty",
			wantOK:   false,
		},
		{
			name:     "a refusal behind a valid header is still a refusal",
			in:       "chore: i cannot describe this diff",
			wantRule: "subject-preamble",
			wantOK:   false,
		},
		{
			name:     "an apology behind a valid header is still an apology",
			in:       "feat(git): i'm sorry, the diff is too large",
			wantRule: "subject-preamble",
			wantOK:   false,
		},
		{
			// The model continued the prompt instead of answering it.
			name:     "a subject carrying an instruction fragment is rejected",
			in:       "feat(shellapp): staged files (3): add the type picker",
			wantRule: "subject-echo",
			wantOK:   false,
		},
		{
			name:     "a body echoing the prompt is rejected",
			in:       "feat(shellapp): add the type picker\n\nbranch: main\nstaged files (2):\nM ui/view.go (+3 -1)",
			wantRule: "body-echo",
			wantOK:   true, // the echoed lines are dropped; the header stands
		},
		{
			name:         "a dangling backtick from a cut-short subject is removed",
			in:           "feat(shellapp): improve draft to `im",
			wantRule:     "",
			wantRepaired: "feat(shellapp): improve draft to im",
			wantOK:       true,
		},
		{
			name:         "body is an AI disclaimer",
			in:           "feat: add fold all\n\nAs an AI language model, I cannot verify the diff.",
			wantRule:     "body-preamble",
			wantRepaired: "feat: add fold all",
			wantOK:       true,
		},
		{
			name:         "body only restates the subject",
			in:           "feat: add fold all\n\nAdd fold all",
			wantRule:     "body-restates-subject",
			wantRepaired: "feat: add fold all",
			wantOK:       true,
		},
	}

	rules := DefaultRules()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.in, rules)
			switch {
			case tt.wantRule == "" && err != nil:
				t.Fatalf("Validate(%q) = %v, want nil", tt.in, err)
			case tt.wantRule != "" && err == nil:
				t.Fatalf("Validate(%q) = nil, want %s", tt.in, tt.wantRule)
			case tt.wantRule != "" && !strings.HasPrefix(err.Error(), tt.wantRule):
				t.Fatalf("Validate(%q) = %v, want rule %s", tt.in, err, tt.wantRule)
			}

			got, ok := Repair(tt.in, rules, "feat")
			if ok != tt.wantOK {
				t.Fatalf("Repair(%q) ok = %v, want %v (got %q)", tt.in, ok, tt.wantOK, got)
			}
			if tt.wantRepaired != "" && got != tt.wantRepaired {
				t.Errorf("Repair(%q)\n got %q\nwant %q", tt.in, got, tt.wantRepaired)
			}
			if ok {
				if err := Validate(got, rules); err != nil {
					t.Errorf("Repair reported ok but Validate(%q) = %v", got, err)
				}
			}
		})
	}
}

func TestValidateHeaderMaxLength(t *testing.T) {
	rules := DefaultRules()
	subject := strings.TrimSpace(strings.Repeat("add another thing ", 8)) // 143 chars
	msg := "feat: " + subject

	err := Validate(msg, rules)
	if err == nil || !strings.HasPrefix(err.Error(), "header-max-length") {
		t.Fatalf("Validate(140-char subject) = %v, want header-max-length", err)
	}

	got, ok := Repair(msg, rules, "feat")
	if !ok {
		t.Fatalf("Repair(%q) not ok: %q", msg, got)
	}
	if len(got) > rules.MaxHeader {
		t.Errorf("repaired header is %d chars, limit %d: %q", len(got), rules.MaxHeader, got)
	}
	if strings.HasSuffix(got, " ") || !strings.HasSuffix(got, "thing") && !strings.HasSuffix(got, "another") && !strings.HasSuffix(got, "add") {
		t.Errorf("header was not cut at a word boundary: %q", got)
	}
}

func TestRepairFallsBackToChoreWithoutAUsableType(t *testing.T) {
	got, ok := Repair("nonsense: something happened", DefaultRules(), "")
	if !ok || got != "chore: something happened" {
		t.Fatalf("Repair = %q, %v; want %q, true", got, ok, "chore: something happened")
	}
}

func TestRepairIsIdempotent(t *testing.T) {
	rules := DefaultRules()
	inputs := []string{
		"Feat: Add Fold All.",
		"```\nfeat: add fold all\n\nit collapses every block\n```",
		"Here is the commit message:\n\nfix(git): stop the panel from scrolling",
	}
	for _, in := range inputs {
		once, ok := Repair(in, rules, "feat")
		if !ok {
			t.Fatalf("Repair(%q) not ok", in)
		}
		twice, ok := Repair(once, rules, "feat")
		if !ok || twice != once {
			t.Errorf("Repair not idempotent for %q: %q then %q", in, once, twice)
		}
	}
}

func TestValidateWithoutAConvention(t *testing.T) {
	rules := Rules{MaxHeader: 72, ForbidFullStop: true, BodyLeadingBlank: true}
	if err := Validate("add fold all to the editor", rules); err != nil {
		t.Errorf("plain subject rejected in a non-conventional repo: %v", err)
	}
	if err := Validate("Add fold all.", rules); err == nil {
		t.Error("subject-case/full-stop still apply without a type")
	}
	// The header must survive intact: splitting it would eat "add".
	got, ok := Repair("Add http: support for redirects.", rules, "")
	if !ok || got != "add http: support for redirects" {
		t.Errorf("Repair = %q, %v; want %q, true", got, ok, "add http: support for redirects")
	}
}

func TestGrammar(t *testing.T) {
	rules := DefaultRules()
	all := Grammar(rules, "")
	for _, typ := range rules.Types {
		if !strings.Contains(all, `"`+typ+`"`) {
			t.Errorf("Grammar() is missing type %q", typ)
		}
	}

	picked := Grammar(rules, "fix")
	if !strings.Contains(picked, `type    ::= "fix"`+"\n") {
		t.Errorf("Grammar(picked) did not narrow the type alternation:\n%s", picked)
	}
	if strings.Contains(picked, `"feat"`) {
		t.Errorf("Grammar(picked) still offers other types:\n%s", picked)
	}
	if picked != Grammar(rules, "fix") {
		t.Error("Grammar is not stable for the same rules and type")
	}
	if strings.Contains(picked, `"nonsense"`) {
		t.Error("Grammar accepted a type outside the enum")
	}
	// An unknown pick falls back to the full enum rather than emitting a
	// grammar the server would reject.
	if Grammar(rules, "nonsense") != all {
		t.Error("Grammar(unknown type) should fall back to the full enum")
	}
	for _, want := range []string{"root", "scope", "subject"} {
		if !strings.Contains(all, want+" ") {
			t.Errorf("Grammar() is missing the %q rule:\n%s", want, all)
		}
	}
	// The grammar stops at the header. A body the model cannot write is a
	// body it cannot invent, which is the whole point of the split.
	if strings.Contains(all, "body") || strings.Contains(all, `\n\n`) {
		t.Errorf("the grammar still allows a body:\n%s", all)
	}
}

// The grammar can only stop the sampler at a character count, which lands
// mid-word; Repair is what cuts cleanly.
func TestRepairCutsHeadersAtAWordBoundary(t *testing.T) {
	rules := DefaultRules()
	inputs := []string{
		"feat(shellapp): add the progress bar, the installer and the error reporting path for downloads",
		"fix: stop the panel scrolling past the last row when the viewport is taller than the list of files",
		"feat(gitpanel): add `FoldAll` and `UnfoldAll` to the editor and wire them into the header toggle",
	}
	for _, in := range inputs {
		got, ok := Repair(in, rules, "feat")
		if !ok {
			t.Fatalf("Repair(%q) not ok: %q", in, got)
		}
		header, _, _ := strings.Cut(got, "\n")
		if len(header) > rules.MaxHeader {
			t.Errorf("header is %d chars: %q", len(header), header)
		}
		if strings.Count(header, "`")%2 == 1 {
			t.Errorf("header left an unmatched backtick: %q", header)
		}
		if strings.HasSuffix(header, " ") || strings.HasSuffix(header, ",") || strings.HasSuffix(header, "-") {
			t.Errorf("header ends on a dangling separator: %q", header)
		}
		// The cut must land on a whole word from the input.
		last := header[strings.LastIndexByte(header, ' ')+1:]
		if last != "" && !strings.Contains(in+" ", last+" ") {
			t.Errorf("header ends in a partial word %q: %q", last, header)
		}
	}
}

// The grammar's ceiling is deliberately generous, because a header one word
// too long is repairable and one cut at exactly the ceiling is not.
func TestGrammarLeavesRoomForRepairToCutCleanly(t *testing.T) {
	g := Grammar(DefaultRules(), "feat")
	if !strings.Contains(g, "{3,70}") {
		t.Errorf("subject ceiling should be MaxHeader-2, giving Repair room to cut at a word boundary:\n%s", g)
	}
}

// The bodies a real qwen2.5-coder-3b produced for a single staged file. The
// subjects were good; the bodies described a preview, styling and syntax
// highlighting that do not exist anywhere in the diff.
func TestRejectsConfabulatedBodies(t *testing.T) {
	rules := DefaultRules()
	confabulated := []string{
		"feat(ui): add compose view for commit messages\n\n" +
			"The compose view component allows users to input a commit message, with\n" +
			"a preview of the message and the ability to switch between different\n" +
			"commit types. The component is styled to be minimalistic and easy to use.",
		"feat(ui): add compose view component\n\n" +
			"It supports syntax highlighting and wrapping, and can be resized to fit\n" +
			"the available space, making it easier to write longer messages.",
		"feat(gitpanel): add the picker\n\nThis provides a better user experience.",
	}
	for _, in := range confabulated {
		if err := Validate(in, rules); err == nil {
			t.Errorf("Validate accepted a confabulated body:\n%s", in)
		} else if !strings.HasPrefix(err.Error(), "body-editorial") {
			t.Errorf("want body-editorial, got %v", err)
		}

		got, ok := Repair(in, rules, "feat")
		if !ok {
			t.Fatalf("Repair(%q) not ok: %q", in, got)
		}
		if strings.Contains(got, "\n") {
			t.Errorf("the invented body survived repair:\n%s", got)
		}
		header, _, _ := strings.Cut(in, "\n")
		if got != header {
			t.Errorf("Repair = %q, want the subject alone (%q)", got, header)
		}
	}
}

// The check must not cost a single real message: every phrase on the list was
// verified against 300 commits of this repository's history.
func TestEditorialCheckSparesRealBodies(t *testing.T) {
	real := []string{
		"fix(gitpanel): stop the panel scrolling past the last row\n\n" +
			"The viewport clamped to the row count rather than the row count minus\n" +
			"the visible height, so the final page scrolled into blank space.",
		"feat(texteditor): fold blocks from indentation\n\n" +
			"A line is a header when the next non-blank line is indented further, and\n" +
			"the block runs to the last line still indented past it. Improves on the\n" +
			"previous behaviour, which ended a block at the first blank line.",
		"perf(envfile): read the environment file once per send\n\n" +
			"It was re-read for every interpolated token, which showed up as a better\n" +
			"than 200ms pause on requests with many variables.",
	}
	for _, in := range real {
		if err := Validate(in, DefaultRules()); err != nil {
			t.Errorf("a genuine message was rejected: %v\n%s", err, in)
		}
	}
}
