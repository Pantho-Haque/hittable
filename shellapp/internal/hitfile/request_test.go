package hitfile

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

// A value with a single quote used to end the shell quoting early, so the
// copied curl did not run; headers also came out in random map order.
func TestAsCurlQuotesAndIsStable(t *testing.T) {
	r := &ResolvedRequest{
		Method:  "PUT",
		URL:     "https://x.io/u",
		Headers: map[string]string{"X-B": "2", "X-A": "it's"},
		Body:    `{"name":"O'Brien"}`,
	}
	want := `curl -X PUT -H 'X-A: it'\''s' -H 'X-B: 2' -d '{"name":"O'\''Brien"}' 'https://x.io/u'`
	for i := 0; i < 5; i++ {
		if got := r.AsCurl(); got != want {
			t.Fatalf("AsCurl =\n%s\nwant\n%s", got, want)
		}
	}
}

func TestUnknownVars(t *testing.T) {
	h := &HitFile{
		URL:     "<<BASE>>/users/<<ID>>",
		Headers: map[string]string{"Authorization": "Bearer <<TOKEN>>"},
		Params:  map[string]string{"q": "<<ID>>"},
		Body:    `{"a":"<<NOPE>>"}`,
	}
	got := UnknownVars(h, map[string]string{"BASE": "x", "TOKEN": ""})
	if strings.Join(got, ",") != "ID,NOPE" {
		t.Fatalf("UnknownVars = %v, want [ID NOPE] (an empty value is still defined)", got)
	}
}

func TestExecuteAndCaptureContextCancels(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := ExecuteAndCaptureContext(ctx, &HitFile{Method: "GET", URL: srv.URL}, nil)
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not abort the request")
	}
}
