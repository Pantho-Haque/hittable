package gitpanel

import (
	"errors"
	"strings"
	"testing"
)

// What husky + lint-staged print when ESLint fails a staged file.
const lintStagedOut = ` Running pre-commit checks...
[STARTED] Running tasks for staged files...
[FAILED] eslint --fix [FAILED]
✖ eslint --fix:

/repo/shellapp/internal/exthost/js/host.js
  2147:44  error  A ` + "`require()`" + ` style import is forbidden  @typescript-eslint/no-require-imports

✖ 1 problem (1 error, 0 warnings)
husky - pre-commit script failed (code 1)`

func TestCommitFailureNamesTheError(t *testing.T) {
	got := CommitFailure(lintStagedOut)
	for _, want := range []string{"pre-commit hook", "host.js", "2147:44", "require()", "message is kept"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is missing %q", got, want)
		}
	}
	if strings.Contains(got, "\n") {
		t.Errorf("not one line: %q", got)
	}
	if got := CommitFailure("husky - commit-msg script failed (code 1)\n❌ Your commit message doesn't follow the conventional commit format!"); !strings.Contains(got, "git hook") {
		t.Errorf("commit-msg failure: %q", got)
	}
	if got := CommitFailure("nothing to commit, working tree clean"); got != "commit: nothing to commit, working tree clean" {
		t.Errorf("plain git error: %q", got)
	}
}

func TestRefusedCommitKeepsTheMessage(t *testing.T) {
	p := &Panel{committing: "feat: keep me\n\nbody"}
	p.Done(DoneMsg{Label: "commit", Err: errors.New(lintStagedOut)})
	if p.draft != "feat: keep me\n\nbody" || !p.draftEdited {
		t.Errorf("draft = %q (edited %v)", p.draft, p.draftEdited)
	}
	if !strings.Contains(p.Message, "host.js") {
		t.Errorf("message = %q", p.Message)
	}
	p.committing = "feat: done"
	p.draft = ""
	p.Done(DoneMsg{Label: "commit"})
	if p.draft != "" || p.committing != "" {
		t.Error("a successful commit must not bring the message back")
	}
}
