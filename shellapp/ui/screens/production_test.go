package screens

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/internal/hitfile"
)

// A production environment (HITTABLE_PRODUCTION=true, the same variable the
// web app uses) asks before a state-changing send: the first ctrl+r arms, the
// second sends. A GET goes straight out.
func TestProductionGuardNeedsASecondCtrlR(t *testing.T) {
	var hits atomic.Int32
	m, hd, _ := apiProject(t, func(w http.ResponseWriter, r *http.Request) { hits.Add(1) })
	os.WriteFile(filepath.Join(hd, "env.json"), []byte(`{"BASE_URL":"`+m.EnvData["BASE_URL"]+`","HITTABLE_PRODUCTION":"true"}`), 0o644)
	ch := withProgram(t)

	post := filepath.Join(hd, "create.hit")
	writeHit(t, post, hitfile.HitFile{Method: "POST", URL: "<<BASE_URL>>/users", Headers: map[string]string{}, Params: map[string]string{}})
	m.openFileRaw(post)
	ctrl(m, tea.KeyCtrlR)
	if m.Sending || !strings.Contains(m.StatusBar, "PRODUCTION — press ctrl+r again to send POST") {
		t.Fatalf("first ctrl+r should only arm: Sending=%v status=%q", m.Sending, m.StatusBar)
	}
	ctrl(m, tea.KeyCtrlR)
	if !m.Sending {
		t.Fatal("second ctrl+r did not send")
	}
	awaitResponse(t, m, ch)
	if hits.Load() != 1 {
		t.Fatalf("server hits = %d, want 1", hits.Load())
	}

	get := filepath.Join(hd, "list.hit")
	writeHit(t, get, hitfile.HitFile{Method: "GET", URL: "<<BASE_URL>>/users", Headers: map[string]string{}, Params: map[string]string{}})
	m.openFileRaw(get)
	ctrl(m, tea.KeyCtrlR)
	if !m.Sending {
		t.Fatal("a GET on production should send at once")
	}
	awaitResponse(t, m, ch)
}

// env.local.json, next to env.json and kept out of git, overrides it.
func TestEnvLocalValuesAreSent(t *testing.T) {
	var got atomic.Value
	m, hd, _ := apiProject(t, func(w http.ResponseWriter, r *http.Request) { got.Store(r.Header.Get("Authorization")) })
	os.WriteFile(filepath.Join(hd, "env.local.json"), []byte(`{"TOKEN":"from-local"}`), 0o644)
	ch := withProgram(t)

	hit := filepath.Join(hd, "me.hit")
	writeHit(t, hit, hitfile.HitFile{Method: "GET", URL: "<<BASE_URL>>/me",
		Headers: map[string]string{"Authorization": "Bearer <<TOKEN>>"}, Params: map[string]string{}})
	m.openFileRaw(hit)
	ctrl(m, tea.KeyCtrlR) // reloadEnv picks up env.local.json before sending
	awaitResponse(t, m, ch)
	if got.Load() != "Bearer from-local" {
		t.Fatalf("Authorization = %v, want the env.local.json value", got.Load())
	}
}
