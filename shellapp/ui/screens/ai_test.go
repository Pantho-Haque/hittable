package screens

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	zone "github.com/lrstanley/bubblezone"

	"github.com/hittable/shellapp/internal/appconfig"
	"github.com/hittable/shellapp/internal/hithome"
	"github.com/hittable/shellapp/internal/llmhost"
	"github.com/hittable/shellapp/ui/components/gitpanel"
)

// sandboxAI points the app-state root at a temp dir so these tests can never
// see, or act on, a model actually installed on the machine running them.
func sandboxAI(t *testing.T) {
	t.Helper()
	t.Setenv(hithome.EnvHome, t.TempDir())
	t.Setenv(appconfig.EnvAI, "")
	t.Setenv(appconfig.EnvAIEndpoint, "")
	t.Setenv(appconfig.EnvAIModel, "")
}

// The default state on every machine: nothing installed, so the panel must get
// a nil Drafter and fall back to the heuristic draft.
func TestNoModelMeansNoDrafter(t *testing.T) {
	sandboxAI(t)
	m, _ := newTestScreen(t)
	if m.Git.Drafter != nil {
		t.Error("Drafter should be nil with nothing installed")
	}
	if m.AI == nil || m.AI.live {
		t.Error("AI should not report live with nothing installed")
	}
}

// The wiring that makes `hittable model enable` mean anything: when a model is
// reachable the panel is handed a real drafter. Proven through the endpoint
// override so it needs no 2GB download.
func TestConfiguredEndpointGivesThePanelADrafter(t *testing.T) {
	sandboxAI(t)
	t.Setenv(appconfig.EnvAIEndpoint, "http://127.0.0.1:65535")

	m, _ := newTestScreen(t)
	if m.Git.Drafter == nil {
		t.Fatal("a configured endpoint should give the Git panel a drafter")
	}
	if !m.AI.live {
		t.Error("AI should report live with an endpoint configured")
	}
}

// The off switch has to win over everything, including a configured endpoint.
// This is what keeps CI and a suspicious user in control.
func TestAIOffBeatsAConfiguredEndpoint(t *testing.T) {
	sandboxAI(t)
	t.Setenv(appconfig.EnvAIEndpoint, "http://127.0.0.1:65535")
	t.Setenv(appconfig.EnvAI, "0")

	m, _ := newTestScreen(t)
	if m.Git.Drafter != nil {
		t.Error("HITTABLE_AI=0 must win over a configured endpoint")
	}
}

// Enabling a model from the integrated terminal has to take effect in the
// running app without a restart. The real trigger is files landing on disk,
// not an env var changing, so this creates what `hittable model enable`
// creates and drives one tick of the poll that already runs git status.
func TestModelAppearsWithoutARestart(t *testing.T) {
	sandboxAI(t)
	m, _ := newTestScreen(t)
	if m.Git.Drafter != nil {
		t.Fatal("precondition: no drafter")
	}

	install := func() {
		t.Helper()
		for _, p := range []string{llmhost.ServerPath(), llmhost.ModelPath()} {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	install()
	if !llmhost.Installed() {
		t.Fatal("fixture did not satisfy llmhost.Installed()")
	}

	msg := m.refreshAI()
	if m.Git.Drafter == nil {
		t.Error("the panel should have picked up the model without a restart")
	}
	if msg == "" {
		t.Error("the transition should surface a status line")
	}

	// And back off again, the way `hittable model disable` would.
	os.Remove(llmhost.ModelPath())
	if msg := m.refreshAI(); msg == "" || m.Git.Drafter != nil {
		t.Errorf("disable should drop the drafter and say so; got %q, drafter=%v", msg, m.Git.Drafter != nil)
	}
}

// A steady state must not spam the status bar every 2 seconds.
func TestRefreshAIIsQuietWhenNothingChanged(t *testing.T) {
	sandboxAI(t)
	m, _ := newTestScreen(t)
	for i := 0; i < 3; i++ {
		if msg := m.refreshAI(); msg != "" {
			t.Fatalf("tick %d reported %q with no change", i, msg)
		}
	}
}

// Startup must touch no network: an endpoint is configured and pointed at a
// live server, and constructing the screen must still never call it. Probing
// on launch would add latency for every user, almost none of whom have a model.
func TestStartupDoesNoNetworkIO(t *testing.T) {
	sandboxAI(t)
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	t.Setenv(appconfig.EnvAIEndpoint, srv.URL)

	tmp := t.TempDir()
	hd := filepath.Join(tmp, "hittable")
	os.MkdirAll(hd, 0o755)
	m := NewMainScreen(tmp, zone.New())
	m.SetSize(120, 40)
	_ = m.View()

	if hits != 0 {
		t.Errorf("startup made %d request(s) to the model endpoint; want 0", hits)
	}
}

// The Send hook must be wired as a closure that checks teaProgram when a
// message is sent, not when the hook is built. main.go calls NewApp before
// SetTeaProgram, so deciding at wire time leaves Send nil forever — and a nil
// Send makes the panel generate inline on the UI goroutine, which freezes the
// whole app for the length of a generation with no spinner and no cancel.
// That shipped once; this is here so it cannot ship again quietly.
func TestGitSendHookSurvivesBeingWiredBeforeTheProgram(t *testing.T) {
	sandboxAI(t)
	SetTeaProgram(nil) // the state NewApp is constructed in
	m, _ := newTestScreen(t)

	if m.Git.Send == nil {
		t.Fatal("Git.Send is nil: generation would run synchronously on the UI goroutine")
	}
	// Sending before a program exists must be a no-op, never a panic.
	m.Git.Send(gitpanel.GenChunkMsg{Seq: 1})
}
