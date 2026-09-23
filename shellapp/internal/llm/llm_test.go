package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// setup keeps every test hermetic: the whole app state tree points at a temp
// dir, so nothing here can read or write a real ~/.hittable even by accident.
func setup(t *testing.T) {
	t.Helper()
	t.Setenv("HITTABLE_HOME", t.TempDir())
}

// endpointOf is the Config.Endpoint a test uses: a closure over the httptest
// server's URL, standing in for the supervisor that picks a port at spawn time.
func endpointOf(srv *httptest.Server) func(context.Context) (string, error) {
	return func(context.Context) (string, error) { return srv.URL, nil }
}

// newClient wires a client to a handler, with timeouts short enough that a
// hung test fails fast rather than waiting out the package timeout.
func newClient(t *testing.T, h http.HandlerFunc) (*HTTPClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	return New(Config{
		Endpoint:    endpointOf(srv),
		Timeout:     5 * time.Second,
		FastTimeout: 2 * time.Second,
	}), srv
}

// silentServer accepts a request and never answers it, which is the failure
// mode our own deadline exists for. It is released at the end of the test
// rather than relying on the server noticing the client hang up: the cleanup
// closing the channel is registered after the one closing the server, so it
// runs first and Close never waits on a live handler.
func silentServer(t *testing.T) *httptest.Server {
	t.Helper()

	stop := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		select {
		case <-stop:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(stop) })
	return srv
}

// flusher writes and flushes, so a test controls exactly how a response body is
// split into TCP writes.
type flusher struct {
	t *testing.T
	w http.ResponseWriter
	f http.Flusher
}

func newFlusher(t *testing.T, w http.ResponseWriter) *flusher {
	t.Helper()
	f, ok := w.(http.Flusher)
	if !ok {
		t.Fatal("ResponseWriter is not a Flusher")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	return &flusher{t: t, w: w, f: f}
}

func (fl *flusher) write(s string) {
	io.WriteString(fl.w, s)
	fl.f.Flush()
}

func unmarshalFrame(s string, f *frame) error {
	return json.Unmarshal([]byte(s), f)
}

// collector records streamed chunks.
type collector struct {
	chunks []string
	done   int
}

func (c *collector) fn(ch Chunk) {
	if ch.Done {
		c.done++
		return
	}
	c.chunks = append(c.chunks, ch.Text)
}

func TestNewAppliesDefaults(t *testing.T) {
	setup(t)

	c := New(Config{})
	if c.cfg.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", c.cfg.Timeout, DefaultTimeout)
	}
	if c.cfg.FastTimeout != DefaultFastTimeout {
		t.Errorf("FastTimeout = %v, want %v", c.cfg.FastTimeout, DefaultFastTimeout)
	}
	if cap(c.sem) != DefaultMaxInFlight {
		t.Errorf("semaphore capacity = %d, want %d", cap(c.sem), DefaultMaxInFlight)
	}
	if c.cfg.HTTPClient == nil || c.cfg.Now == nil {
		t.Error("New left the http client or the clock nil")
	}
}

// Building a client must not ask for an endpoint: resolving one may spawn a
// 3GB process, and nothing about constructing a client is a decision to do so.
func TestNewAsksForNoEndpoint(t *testing.T) {
	setup(t)

	asked := 0
	New(Config{Endpoint: func(context.Context) (string, error) {
		asked++
		return "http://127.0.0.1:1", nil
	}})
	if asked != 0 {
		t.Errorf("New asked for the endpoint %d times, want 0", asked)
	}
}
