package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The whole request loop through a real terminal: click the request in the
// explorer, press ctrl+r (0x12), see the status on screen, and find the
// response saved in the .hit by autosave — with the env template intact.
func TestSendRequestThroughRealTerminal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Key") != "k-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"created":"e2e-marker"}`)
	}))
	defer srv.Close()

	h, hd := start(t, 120, 40)
	env, _ := json.Marshal(map[string]string{"HOST": srv.URL, "KEY": "k-123"})
	os.WriteFile(filepath.Join(hd, "env.json"), env, 0o644)
	hit := filepath.Join(hd, "a.hit")
	os.WriteFile(hit, []byte(`{"method":"POST","url":"<<HOST>>/things","headers":{"X-Key":"<<KEY>>"},"params":{},"body":"{}","response":null}`), 0o644)

	x, y, ok := h.find("hittable/")
	if !ok {
		t.Fatalf("explorer not rendered:\n%s", strings.Join(h.screen(), "\n"))
	}
	h.click(x, y)
	x, y, ok = h.find("a.hit")
	if !ok {
		t.Fatal("a.hit not visible")
	}
	h.click(x, y)
	if _, _, ok := h.find("POST"); !ok {
		t.Fatalf("runner not open:\n%s", strings.Join(h.screen(), "\n"))
	}
	fmt.Fprint(h.in, "\x12") // ctrl+r
	if _, _, ok := h.find("201 Created"); !ok {
		t.Fatalf("status not shown:\n%s", strings.Join(h.screen(), "\n"))
	}
	if _, _, ok := h.find("e2e-marker"); !ok {
		t.Fatalf("body not shown:\n%s", strings.Join(h.screen(), "\n"))
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		b, _ := os.ReadFile(hit)
		if strings.Contains(string(b), "e2e-marker") {
			if !strings.Contains(string(b), "<<HOST>>/things") || strings.Contains(string(b), srv.URL) {
				t.Fatalf("the saved file lost its template:\n%s", b)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("response never autosaved:\n%s", b)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
