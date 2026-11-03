package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The Extensions panel through the real terminal: alt+x opens it, typing
// searches with the list staying on screen (never a blank frame), the
// selected extension's page renders, and esc clears then closes.
func TestExtensionsPanelThroughRealTerminal(t *testing.T) {
	t.Setenv("HITTABLE_HOME", t.TempDir())
	slowSearch := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e := func(name, display string) map[string]any {
			return map[string]any{"namespace": "acme", "name": name, "displayName": display, "description": display + " for tests",
				"version": "1.0.0", "downloadCount": 1234567, "averageRating": 4.0, "verified": true}
		}
		switch r.URL.Path {
		case "/api/-/search":
			if r.URL.Query().Get("query") != "" {
				<-slowSearch // hold the answer so the loading state is visible
				json.NewEncoder(w).Encode(map[string]any{"extensions": []any{e("zeta", "Zeta Tools")}})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"extensions": []any{e("alpha", "Alpha Lint"), e("beta", "Beta Format")}})
		case "/api/acme/alpha", "/api/acme/beta", "/api/acme/zeta":
			d := e(strings.TrimPrefix(r.URL.Path, "/api/acme/"), "Page")
			d["files"] = map[string]string{"readme": "http://" + r.Host + "/readme"}
			json.NewEncoder(w).Encode(d)
		case "/readme":
			fmt.Fprint(w, "# Readme heading\n\nREADME-MARKER text.\n")
		}
	}))
	defer srv.Close()
	t.Setenv("HITTABLE_EXT_REGISTRY", srv.URL)

	h, _ := start(t, 140, 40)
	dump := func() string { return strings.Join(h.screen(), "\n") }
	fmt.Fprint(h.in, "\x1bx") // alt+x
	if _, _, ok := h.find("Alpha Lint"); !ok {
		t.Fatalf("panel did not list the popular extensions:\n%s", dump())
	}
	if _, _, ok := h.find("README-MARKER"); !ok {
		t.Fatalf("the selected extension's README did not render:\n%s", dump())
	}

	fmt.Fprint(h.in, "zeta")
	if _, _, ok := h.find("Searching Open VSX"); !ok {
		t.Fatalf("no searching state:\n%s", dump())
	}
	// While the search is out, the panel (and its previous list) stays.
	for i := 0; i < 10; i++ {
		if _, _, ok := h.find("Alpha Lint"); !ok {
			t.Fatalf("the list vanished while searching:\n%s", dump())
		}
		time.Sleep(30 * time.Millisecond)
	}
	close(slowSearch)
	if _, _, ok := h.find("Zeta Tools"); !ok {
		t.Fatalf("search results did not show:\n%s", dump())
	}
	fmt.Fprint(h.in, "\x1b") // esc: clear the search
	if _, _, ok := h.find("Alpha Lint"); !ok {
		t.Fatalf("esc should clear the search back to the popular list:\n%s", dump())
	}
	time.Sleep(100 * time.Millisecond)
	fmt.Fprint(h.in, "\x1b") // esc: close
	if !h.gone("POPULAR ON OPEN VSX") {
		t.Fatalf("esc should close the panel:\n%s", dump())
	}
}
