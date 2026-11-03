package gitpanel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	tea "github.com/charmbracelet/bubbletea"
)

// A pre-commit hook that fails, printing to both stdout and stderr: the
// panel shows all of it, in order, with the command and a hint.
func TestFailedCommitShowsTheHookOutput(t *testing.T) {
	r := composeRepo(t) // has a staged file
	hook := filepath.Join(r.Root, ".git", "hooks", "pre-commit")
	script := "#!/bin/sh\necho 'Running pre-commit checks...'\necho '/repo/src/app.js' >&2\necho '  12:4  error  Unexpected console statement  no-console' >&2\necho 'husky - pre-commit script failed (code 1)' >&2\nexit 1\n"
	if err := os.WriteFile(hook, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	p := New(r)
	p.SetSize(140, 30)
	p.Refresh()
	p.committing = "feat(core): add the drafter"
	p.async("commit", func() (string, error) { return "", r.Commit(p.committing) })

	if !p.ShowOutput {
		t.Fatal("a failed commit should open the output view")
	}
	body := ansi.Strip(strings.Join(p.detailLines(), "\n"))
	for _, want := range []string{"✖ commit failed (exit 1)", "$ git commit -m <message>", "pre-commit hook refused",
		"Running pre-commit checks...", "12:4  error  Unexpected console statement", "husky - pre-commit script failed"} {
		if !strings.Contains(body, want) {
			t.Errorf("output view is missing %q:\n%s", want, body)
		}
	}
	// stdout and stderr in the order they were printed.
	if strings.Index(body, "Running pre-commit") > strings.Index(body, "Unexpected console") {
		t.Error("output is out of order")
	}
	if p.draft != "feat(core): add the drafter" {
		t.Errorf("message not kept: %q", p.draft)
	}
	if v := ansi.Strip(p.View(zone.New())); !strings.Contains(v, "Unexpected console statement") {
		t.Errorf("not on screen:\n%s", v)
	}

	// esc goes back to the diff; ! brings the output back.
	p.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if p.ShowOutput {
		t.Error("esc should close the output view")
	}
	p.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	if !p.ShowOutput {
		t.Error("! should reopen it")
	}
	// Scrolling keys stay in the view.
	p.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	if !p.ShowOutput {
		t.Error("moving in the output closed it")
	}
}
