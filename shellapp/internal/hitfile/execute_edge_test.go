package hitfile

import (
	"bytes"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResolveSchemeAndTemplates(t *testing.T) {
	env := map[string]string{"HOST": "api.test", "ID": "7", "EMPTY": ""}
	cases := []struct{ in, want string }{
		{"  localhost:8080/x  ", "http://localhost:8080/x"},
		{"https://a.test/<<ID>>", "https://a.test/7"},
		{"<<HOST>>/v1", "http://api.test/v1"},
		{"<<MISSING>>/v1", "http://<<MISSING>>/v1"}, // unknown keys are left visible
		{"a.test/<<EMPTY>>x", "http://a.test/x"},
		{"a.test/<<ID", "http://a.test/<<ID"}, // unterminated template kept verbatim
		{"a.test/<<>>", "http://a.test/<<>>"},
		{"a.test/<<<ID>>>", "http://a.test/<<<ID>>>"}, // key is "<ID": not a template
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		if got := Resolve(&HitFile{URL: c.in}, env).URL; got != c.want {
			t.Errorf("Resolve(%q).URL = %q, want %q", c.in, got, c.want)
		}
	}
	r := Resolve(&HitFile{
		URL:     "x.test",
		Headers: map[string]string{"Authorization": "Bearer <<ID>>"},
		Params:  map[string]string{"id": "<<ID>>"},
		Body:    `{"id":"<<ID>>","u":"<<MISSING>>"}`,
	}, env)
	if r.Headers["Authorization"] != "Bearer 7" || r.Params["id"] != "7" || r.Body != `{"id":"7","u":"<<MISSING>>"}` {
		t.Errorf("resolved: %+v", r)
	}
	// nil maps from a hand-written file must not panic.
	if r := Resolve(&HitFile{URL: "x.test"}, nil); r.URL != "http://x.test" {
		t.Errorf("nil env: %+v", r)
	}
}

func TestFullURLEncodesAndAppends(t *testing.T) {
	r := &ResolvedRequest{URL: "http://a.test/p?keep=1", Params: map[string]string{"q": "a&b=c d", "z": ""}}
	if got := r.FullURL(); got != "http://a.test/p?keep=1&q=a%26b%3Dc+d&z=" {
		t.Errorf("FullURL = %q", got)
	}
	if got := (&ResolvedRequest{URL: "http://a.test"}).FullURL(); got != "http://a.test" {
		t.Errorf("no params = %q", got)
	}
}

// A large pasted body must interpolate in linear time, not freeze the send.
func TestInterpolateLargeBodyIsFast(t *testing.T) {
	body := strings.Repeat(`{"k":"<<V>>","pad":"xxxxxxxxxxxxxxxx"},`, 50000) // ~2MB
	start := time.Now()
	got := interpolateString(body, map[string]string{"V": "v"})
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("interpolating %d bytes took %v", len(body), d)
	}
	if strings.Contains(got, "<<V>>") || !strings.HasPrefix(got, `{"k":"v",`) {
		t.Errorf("not interpolated: %.60s", got)
	}
}

func TestInterpolateUnicode(t *testing.T) {
	got := interpolateString("héllo <<NAME>> 🚀 <<X", map[string]string{"NAME": "wörld"})
	if got != "héllo wörld 🚀 <<X" {
		t.Errorf("got %q", got)
	}
}

func TestExecuteEveryMethodAndBody(t *testing.T) {
	var gotMethod, gotBody, gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotMethod, gotBody, gotCT = r.Method, string(b), r.Header.Get("Content-Type")
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
		h := &HitFile{Method: m, URL: srv.URL, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"a":1}`}
		resp, err := ExecuteAndCapture(h, nil)
		if err != nil {
			t.Fatalf("%s: %v", m, err)
		}
		if gotMethod != m || gotCT != "application/json" || resp.Status != 200 {
			t.Errorf("%s: server saw %s ct=%q status %d", m, gotMethod, gotCT, resp.Status)
		}
		if m != "HEAD" && (gotBody != `{"a":1}` || resp.Data != "ok") {
			t.Errorf("%s: body %q data %#v", m, gotBody, resp.Data)
		}
	}
	// An empty method in a hand-written file is sent as GET.
	if _, err := ExecuteAndCapture(&HitFile{URL: srv.URL}, nil); err != nil || gotMethod != "GET" {
		t.Errorf("empty method: %s %v", gotMethod, err)
	}
}

func TestExecuteErrorsAreReturnedNotPanics(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	refused := "http://" + l.Addr().String()
	l.Close()
	for _, u := range []string{refused, "http://[::1", "://x", "http://exa mple.test"} {
		if _, err := ExecuteAndCapture(&HitFile{Method: "GET", URL: u}, nil); err == nil {
			t.Errorf("%q: want error", u)
		}
	}
	if _, err := ExecuteAndCapture(&HitFile{Method: "BAD METHOD", URL: "http://127.0.0.1"}, nil); err == nil {
		t.Error("invalid method: want error")
	}
}

func TestExecuteNon2xxRedirectAndHeaders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/gone", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"not found"}`))
	})
	mux.HandleFunc("/boom", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) })
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new?x=1", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(r.URL.RawQuery)) })
	mux.HandleFunc("/nocontent", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := ExecuteAndCapture(&HitFile{Method: "GET", URL: srv.URL + "/gone"}, nil)
	if err != nil || resp.Status != 404 || resp.Ok || resp.StatusText != "Not Found" {
		t.Fatalf("404: %+v %v", resp, err)
	}
	if m, _ := resp.Data.(map[string]interface{}); m["error"] != "not found" || resp.Headers["Content-Type"] != "application/json" {
		t.Errorf("404 body/headers: %#v %v", resp.Data, resp.Headers)
	}
	if resp, err := ExecuteAndCapture(&HitFile{Method: "GET", URL: srv.URL + "/boom"}, nil); err != nil || resp.Status != 500 || resp.Ok || resp.Data != "boom\n" {
		t.Errorf("500: %+v %v", resp, err)
	}
	if resp, err := ExecuteAndCapture(&HitFile{Method: "GET", URL: srv.URL + "/old"}, nil); err != nil || resp.Status != 200 || resp.Data != "x=1" {
		t.Errorf("redirect: %+v %v", resp, err)
	}
	if resp, err := ExecuteAndCapture(&HitFile{Method: "DELETE", URL: srv.URL + "/nocontent"}, nil); err != nil || resp.Status != 204 || !resp.Ok || resp.Data != "" || resp.SizeBytes != 0 {
		t.Errorf("204: %+v %v", resp, err)
	}
}

func TestExecuteGzip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Write([]byte(`{"plain":true}`))
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		zw := gzip.NewWriter(w)
		zw.Write([]byte(`{"zipped":true}`))
		zw.Close()
	}))
	defer srv.Close()
	// Without the header Go negotiates gzip itself and decodes.
	resp, err := ExecuteAndCapture(&HitFile{Method: "GET", URL: srv.URL}, nil)
	if m, _ := resp.Data.(map[string]interface{}); err != nil || m["zipped"] != true {
		t.Errorf("auto gzip: %#v %v", resp.Data, err)
	}
	// Headers copied from a browser/Postman carry Accept-Encoding; the
	// response must still be readable, not compressed bytes.
	resp, err = ExecuteAndCapture(&HitFile{Method: "GET", URL: srv.URL, Headers: map[string]string{"Accept-Encoding": "gzip, deflate"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := resp.Data.(map[string]interface{}); m["zipped"] != true {
		t.Errorf("explicit Accept-Encoding: body not decoded: %q", resp.Data)
	}
}

func TestExecuteCookiesBinaryAndLarge(t *testing.T) {
	big := bytes.Repeat([]byte("y"), 4<<20)
	bin := []byte{0x00, 0x01, 0xff, 0xfe, 0x80}
	mux := http.NewServeMux()
	mux.HandleFunc("/cookies", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "a", Value: "1"})
		http.SetCookie(w, &http.Cookie{Name: "b", Value: "2", HttpOnly: true})
	})
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) { w.Write(bin) })
	mux.HandleFunc("/big", func(w http.ResponseWriter, r *http.Request) { w.Write(big) })
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := ExecuteAndCapture(&HitFile{Method: "GET", URL: srv.URL + "/cookies"}, nil)
	if err != nil || resp.Cookies["a"] != "1" || resp.Cookies["b"] != "2" {
		t.Errorf("cookies: %+v %v", resp, err)
	}
	resp, err = ExecuteAndCapture(&HitFile{Method: "GET", URL: srv.URL + "/bin"}, nil)
	if err != nil || resp.SizeBytes != len(bin) || resp.Data != string(bin) {
		t.Errorf("binary: %+v %v", resp, err)
	}
	// Binary data must still marshal into the .hit file without error.
	if out := Marshal(&HitFile{Response: resp}); len(out) == 0 {
		t.Error("binary response did not marshal")
	}
	resp, err = ExecuteAndCapture(&HitFile{Method: "GET", URL: srv.URL + "/big"}, nil)
	if err != nil || resp.SizeBytes != len(big) {
		t.Errorf("big: %d %v", resp.SizeBytes, err)
	}
	resp, err = ExecuteAndCapture(&HitFile{Method: "POST", URL: srv.URL + "/echo", Body: string(big)}, nil)
	if err != nil || resp.SizeBytes != len(big) {
		t.Errorf("big upload: %d %v", resp.SizeBytes, err)
	}
}

func TestParseMalformed(t *testing.T) {
	for _, in := range []string{"", "{", "null", "[]", `{"method":1}`, `{"headers":[]}`, "\x00\xff", `{"url":"x"}garbage`} {
		h, err := Parse([]byte(in))
		if err == nil && h == nil {
			t.Errorf("%q: nil file without error", in)
		}
		if err == nil && (h.Headers == nil || h.Params == nil) {
			t.Errorf("%q: maps not initialised", in)
		}
	}
	// Missing fields are filled so callers can range/assign without nil checks.
	h, err := Parse([]byte(`{"url":"x"}`))
	if err != nil || h.Headers == nil || h.Params == nil {
		t.Fatalf("partial: %+v %v", h, err)
	}
	h.Headers["k"] = "v"
}

func TestAsCurlAndDump(t *testing.T) {
	r := &ResolvedRequest{Method: "POST", URL: "http://a.test/x", Headers: map[string]string{"X-A": "1"}, Params: map[string]string{"q": "1"}, Body: "hi"}
	c := r.AsCurl()
	for _, want := range []string{"curl -X POST", "-H 'X-A: 1'", "-d 'hi'", "'http://a.test/x?q=1'"} {
		if !strings.Contains(c, want) {
			t.Errorf("curl missing %q: %s", want, c)
		}
	}
	req, err := r.ToHTTPRequest()
	if err != nil {
		t.Fatal(err)
	}
	if d := DumpRequest(req); !strings.Contains(d, "POST /x?q=1") || !strings.Contains(d, "hi") {
		t.Errorf("dump: %s", d)
	}
}
