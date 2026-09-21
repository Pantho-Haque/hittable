package hitfile

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecuteAndCaptureEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "a b" || r.Header.Get("X-Token") != "secret" || r.Method != "POST" {
			t.Errorf("bad request: %s %s %v", r.Method, r.URL, r.Header)
		}
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "1"})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	h := &HitFile{
		Method:  "POST",
		URL:     "<<BASE>>/items",
		Headers: map[string]string{"X-Token": "<<TOKEN>>"},
		Params:  map[string]string{"q": "a b"},
		Body:    `{"x":1}`,
	}
	env := map[string]string{"BASE": strings.TrimPrefix(srv.URL, "http://"), "TOKEN": "secret"}
	resp, err := ExecuteAndCapture(h, env)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 201 || !resp.Ok || resp.StatusText != "Created" {
		t.Errorf("status: %+v", resp)
	}
	if m, ok := resp.Data.(map[string]interface{}); !ok || m["ok"] != true {
		t.Errorf("data: %#v", resp.Data)
	}
	if resp.Cookies["sid"] != "1" || resp.SizeBytes != 11 {
		t.Errorf("cookies/size: %+v", resp)
	}
	if h.URL != "<<BASE>>/items" {
		t.Errorf("template mutated: %s", h.URL)
	}
	if curl := Resolve(h, env).AsCurl(); !strings.Contains(curl, "q=a+b") || !strings.Contains(curl, "-X POST") {
		t.Errorf("curl: %s", curl)
	}
}
