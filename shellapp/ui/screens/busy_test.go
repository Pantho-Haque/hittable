package screens

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	zone "github.com/lrstanley/bubblezone"
)

// A push runs in the background, so the UI has to show it is working. The
// spinner used to tick only for request sends, leaving git operations frozen.
func TestGitOperationAnimatesTheSpinner(t *testing.T) {
	// The panel needs a repository, or it short-circuits to "Not a git
	// repository" and never renders its footer.
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "hittable"), 0o755)
	for _, args := range [][]string{{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@x"}, {"config", "user.name", "T"}} {
		if err := exec.Command("git", append([]string{"-C", tmp}, args...)...).Run(); err != nil {
			t.Skipf("git unavailable: %v", err)
		}
	}

	z := zone.New()
	m := NewMainScreen(tmp, z)
	m.SetSize(140, 40)

	if m.busy() {
		t.Fatal("idle screen reports itself busy")
	}
	// Nothing in flight: a stray tick must not keep the loop alive.
	if _, cmd := m.Update(spinner.TickMsg{}); cmd != nil {
		t.Error("spinner kept ticking while idle")
	}

	m.Git.Busy = "push…"
	_, cmd := m.Update(nil)
	if cmd == nil {
		t.Fatal("a git operation did not start the spinner")
	}
	if !m.spinning {
		t.Error("spinner loop was not marked as running")
	}

	// The frame reaches the panel, which is where the label lives.
	m.GitOpen = true
	m.SetSize(140, 40)
	view := z.Scan(m.View())
	frame := strings.TrimSpace(m.Spinner.View())
	if frame == "" || !strings.Contains(view, frame) {
		t.Errorf("spinner frame %q missing from the rendered view", frame)
	}
	if !strings.Contains(view, "push") {
		t.Error("the operation label is not shown")
	}

	// And it stops once the operation finishes.
	m.Git.Busy = ""
	if _, cmd := m.Update(spinner.TickMsg{}); cmd != nil {
		t.Error("spinner still ticking after the operation finished")
	}
	if m.spinning {
		t.Error("spinner loop not released")
	}
}
