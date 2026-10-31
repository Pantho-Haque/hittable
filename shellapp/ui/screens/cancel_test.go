package screens

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/internal/hitfile"
)

// Esc while a request is in flight cancels it (the web app's Cancel), rather
// than closing the file under a request that is still running.
func TestEscCancelsAnInFlightRequest(t *testing.T) {
	release := make(chan struct{})
	m, hd, _ := apiProject(t, func(w http.ResponseWriter, r *http.Request) { <-release })
	t.Cleanup(func() { close(release) })
	ch := withProgram(t)

	hit := filepath.Join(hd, "slow.hit")
	writeHit(t, hit, hitfile.HitFile{Method: "GET", URL: "<<BASE_URL>>/slow", Headers: map[string]string{}, Params: map[string]string{}})
	m.openFileRaw(hit)
	ctrl(m, tea.KeyCtrlR)
	if !m.Sending || !strings.Contains(m.StatusBar, "esc cancels") {
		t.Fatalf("not sending: Sending=%v status=%q", m.Sending, m.StatusBar)
	}

	start := time.Now()
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	awaitResponse(t, m, ch)
	if time.Since(start) > 2*time.Second {
		t.Fatal("cancel did not abort the request promptly")
	}
	if m.Sending || m.StatusBar != "Request cancelled" {
		t.Fatalf("after cancel: Sending=%v status=%q", m.Sending, m.StatusBar)
	}
	if m.ActiveFile != hit {
		t.Fatal("esc closed the file instead of only cancelling the request")
	}
}

// A <<KEY>> env.json does not define is sent as literal text; the status line
// names it so a typo does not pass unnoticed.
func TestUnknownVariablesAreNamedAfterSending(t *testing.T) {
	m, hd, _ := apiProject(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	ch := withProgram(t)

	hit := filepath.Join(hd, "vars.hit")
	writeHit(t, hit, hitfile.HitFile{Method: "GET", URL: "<<BASE_URL>>/u/<<USER_ID>>",
		Headers: map[string]string{"Authorization": "Bearer <<TOKEN>>"}, Params: map[string]string{}})
	m.openFileRaw(hit)
	ctrl(m, tea.KeyCtrlR)
	awaitResponse(t, m, ch)
	if !strings.Contains(m.StatusBar, "unknown <<USER_ID>> sent as written") || strings.Contains(m.StatusBar, "TOKEN") {
		t.Fatalf("status = %q", m.StatusBar)
	}
}
