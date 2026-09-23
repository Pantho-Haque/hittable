package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The route switch is the whole routing rule, so it gets a table: which fields
// are set decides the path, and nothing else does.
func TestRouteSwitch(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		want string
	}{
		{"messages only", Request{Messages: []Message{{Role: RoleUser, Content: "hi"}}}, "/v1/chat/completions"},
		{"system only", Request{System: "you are a commit writer"}, "/v1/chat/completions"},
		{"empty request", Request{}, "/v1/chat/completions"},
		{"prompt", Request{Prompt: "func main"}, "/v1/completions"},
		{"prompt and suffix", Request{Prompt: "func main", Suffix: "}"}, "/infill"},
		{"suffix alone", Request{Suffix: "}"}, "/infill"},
		{"suffix beats messages", Request{Messages: []Message{{Role: RoleUser}}, Prompt: "a", Suffix: "b"}, "/infill"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)

			var got string
			c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"content":"ok","stop":true}`)
			})

			if _, err := c.Complete(context.Background(), tt.req); err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if got != tt.want {
				t.Errorf("path = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompleteParsesEachShape(t *testing.T) {
	tests := []struct {
		name     string
		req      Request
		body     string
		wantText string
		wantStop string
	}{
		{
			name:     "chat",
			req:      Request{Messages: []Message{{Role: RoleUser, Content: "hi"}}},
			body:     `{"choices":[{"message":{"content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1}}`,
			wantText: "hello",
			wantStop: "stop",
		},
		{
			name:     "completions",
			req:      Request{Prompt: "2+2="},
			body:     `{"choices":[{"text":"4","finish_reason":"length"}]}`,
			wantText: "4",
			wantStop: "length",
		},
		{
			name:     "infill",
			req:      Request{Prompt: "func add(", Suffix: ") int {"},
			body:     `{"content":"a, b int","stop":true,"stop_type":"eos","tokens_predicted":5}`,
			wantText: "a, b int",
			wantStop: "eos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)

			c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, tt.body)
			})

			resp, err := c.Complete(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if resp.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", resp.Text, tt.wantText)
			}
			if resp.StopReason != tt.wantStop {
				t.Errorf("StopReason = %q, want %q", resp.StopReason, tt.wantStop)
			}
		})
	}
}

func TestCompleteStreamFlagMatchesTheCall(t *testing.T) {
	setup(t)

	var streamed []bool
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		streamed = append(streamed, body["stream"] == true)

		if body["stream"] == true {
			fl := newFlusher(t, w)
			fl.write("data: [DONE]\n\n")
			return
		}
		io.WriteString(w, `{"content":"x","stop":true}`)
	})

	if _, err := c.Complete(context.Background(), Request{Prompt: "a"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if _, err := c.Stream(context.Background(), Request{Prompt: "a"}, nil); err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if want := []bool{false, true}; !reflect.DeepEqual(streamed, want) {
		t.Errorf("stream flags = %v, want %v", streamed, want)
	}
}

func TestHTTPErrorBodies(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		body       string
		wantMsg    string
		wantSentin error
	}{
		{
			name:    "500 with a JSON body",
			code:    http.StatusInternalServerError,
			body:    `{"error":{"message":"failed to load model","type":"server_error"}}`,
			wantMsg: "failed to load model",
		},
		{
			name:    "500 with a plain JSON error string",
			code:    http.StatusInternalServerError,
			body:    `{"error":"slot unavailable"}`,
			wantMsg: "slot unavailable",
		},
		{
			name:    "500 with an HTML body",
			code:    http.StatusInternalServerError,
			body:    "<html>\n  <head><title>502 Bad Gateway</title></head>\n  <body>nginx</body>\n</html>",
			wantMsg: "<html> <head><title>502 Bad Gateway</title></head> <body>nginx</body> </html>",
		},
		{
			name:       "404 means the model is not there",
			code:       http.StatusNotFound,
			body:       `{"error":{"message":"model not found"}}`,
			wantMsg:    "model not found",
			wantSentin: ErrModelMissing,
		},
		{
			name:       "503 means nothing is answering yet",
			code:       http.StatusServiceUnavailable,
			body:       `{"error":{"message":"loading model"}}`,
			wantMsg:    "loading model",
			wantSentin: ErrUnreachable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)

			c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.code)
				io.WriteString(w, tt.body)
			})

			resp, err := c.Complete(context.Background(), Request{Prompt: "x"})
			if err == nil {
				t.Fatal("Complete returned no error")
			}
			if resp == nil {
				t.Error("Complete returned a nil response for a failed request")
			}

			var se *StatusError
			if !errors.As(err, &se) {
				t.Fatalf("err = %v (%T), want a *StatusError", err, err)
			}
			if se.Code != tt.code {
				t.Errorf("Code = %d, want %d", se.Code, tt.code)
			}
			if se.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", se.Message, tt.wantMsg)
			}
			if tt.wantSentin != nil && !errors.Is(err, tt.wantSentin) {
				t.Errorf("err = %v, want it to wrap %v", err, tt.wantSentin)
			}
			if tt.wantSentin == nil && (errors.Is(err, ErrModelMissing) || errors.Is(err, ErrUnreachable)) {
				t.Errorf("err = %v wrapped a sentinel it should not have", err)
			}
		})
	}
}

// A server that accepts the connection and never writes is the worst failure
// mode there is: without our own deadline the UI waits forever.
func TestDeadlineFiresOnASilentServer(t *testing.T) {
	setup(t)

	srv := silentServer(t)

	c := New(Config{
		Endpoint:    endpointOf(srv),
		Timeout:     120 * time.Millisecond,
		FastTimeout: 40 * time.Millisecond,
	})

	tests := []struct {
		name string
		req  Request
		max  time.Duration
	}{
		{"chat uses Timeout", Request{Messages: []Message{{Role: RoleUser, Content: "x"}}}, 2 * time.Second},
		{"infill uses FastTimeout", Request{Prompt: "a", Suffix: "b"}, time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			_, err := c.Complete(context.Background(), tt.req)
			elapsed := time.Since(start)

			if !errors.Is(err, ErrTimeout) {
				t.Fatalf("err = %v, want ErrTimeout", err)
			}
			if errors.Is(err, context.Canceled) {
				t.Error("a timeout was reported as a cancellation")
			}
			if elapsed > tt.max {
				t.Errorf("took %v, want under %v", elapsed, tt.max)
			}
		})
	}
}

// The infill route must be bounded by FastTimeout, not Timeout: a completion
// that arrives after the user typed the next character is worse than none.
func TestInfillUsesFastTimeout(t *testing.T) {
	setup(t)

	srv := silentServer(t)

	c := New(Config{Endpoint: endpointOf(srv), Timeout: 10 * time.Second, FastTimeout: 50 * time.Millisecond})

	start := time.Now()
	if _, err := c.Complete(context.Background(), Request{Prompt: "a", Suffix: "b"}); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("infill waited %v; it used the slow timeout", elapsed)
	}
}

// A caller that brings its own deadline meant it, so we must not shorten it.
func TestCallerDeadlineWins(t *testing.T) {
	setup(t)

	srv := silentServer(t)

	c := New(Config{Endpoint: endpointOf(srv), Timeout: time.Millisecond, FastTimeout: time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := c.Complete(ctx, Request{Prompt: "x"})
	if err == nil {
		t.Fatal("expected the caller's deadline to fire")
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Errorf("returned after %v; the client's own 1ms timeout overrode the caller's 150ms", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestDisabledWithoutAnEndpoint(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want error
	}{
		{"nil endpoint func", Config{}, ErrDisabled},
		{"empty endpoint", Config{Endpoint: func(context.Context) (string, error) { return "", nil }}, ErrDisabled},
		{"whitespace endpoint", Config{Endpoint: func(context.Context) (string, error) { return "  ", nil }}, ErrDisabled},
		{"endpoint error", Config{Endpoint: func(context.Context) (string, error) { return "", errors.New("spawn failed") }}, ErrUnreachable},
		{"endpoint says disabled", Config{Endpoint: func(context.Context) (string, error) { return "", ErrDisabled }}, ErrDisabled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)
			c := New(tt.cfg)

			if _, err := c.Complete(context.Background(), Request{Prompt: "x"}); !errors.Is(err, tt.want) {
				t.Errorf("Complete err = %v, want %v", err, tt.want)
			}
			if _, err := c.Stream(context.Background(), Request{Prompt: "x"}, nil); !errors.Is(err, tt.want) {
				t.Errorf("Stream err = %v, want %v", err, tt.want)
			}
			if h := c.Probe(context.Background()); h.OK || !errors.Is(h.Err, tt.want) {
				t.Errorf("Probe health = %+v, want an error wrapping %v", h, tt.want)
			}
		})
	}
}

func TestEndpointTrailingSlashIsTrimmed(t *testing.T) {
	setup(t)

	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		io.WriteString(w, `{"content":"x","stop":true}`)
	}))
	t.Cleanup(srv.Close)

	c := New(Config{Endpoint: func(context.Context) (string, error) { return srv.URL + "/", nil }})
	if _, err := c.Complete(context.Background(), Request{Prompt: "x"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if path != "/v1/completions" {
		t.Errorf("path = %q, want %q", path, "/v1/completions")
	}
}

// Probe is cached because the status line asks far more often than the answer
// can change; the handler hit count is the assertion.
func TestProbeIsCached(t *testing.T) {
	setup(t)

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("Probe asked for %q, want /health", r.URL.Path)
		}
		atomic.AddInt32(&hits, 1)
		io.WriteString(w, `{"status":"ok","model":"qwen2.5-coder-3b"}`)
	}))
	t.Cleanup(srv.Close)

	now := time.Now()
	c := New(Config{Endpoint: endpointOf(srv), Now: func() time.Time { return now }})

	for i := range 5 {
		h := c.Probe(context.Background())
		if !h.OK {
			t.Fatalf("probe %d: not OK: %v", i, h.Err)
		}
		if h.Model != "qwen2.5-coder-3b" {
			t.Errorf("Model = %q, want the server's", h.Model)
		}
		if h.Endpoint != srv.URL {
			t.Errorf("Endpoint = %q, want %q", h.Endpoint, srv.URL)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("server saw %d health requests, want 1", got)
	}

	// Just inside the window: still cached.
	now = now.Add(probeTTL - time.Second)
	c.Probe(context.Background())
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("server saw %d health requests inside the window, want 1", got)
	}

	// Past the window: asked again.
	now = now.Add(2 * time.Second)
	c.Probe(context.Background())
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("server saw %d health requests after the window, want 2", got)
	}

	// And an explicit invalidation forces a fresh answer.
	c.InvalidateProbe()
	c.Probe(context.Background())
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("server saw %d health requests after InvalidateProbe, want 3", got)
	}
}

func TestProbeConcurrentCallersIssueOneRequest(t *testing.T) {
	setup(t)

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		io.WriteString(w, `{"status":"ok"}`)
	}))
	t.Cleanup(srv.Close)

	c := New(Config{Endpoint: endpointOf(srv)})

	done := make(chan struct{})
	for range 8 {
		go func() {
			c.Probe(context.Background())
			done <- struct{}{}
		}()
	}
	for range 8 {
		<-done
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("server saw %d health requests from 8 concurrent probes, want 1", got)
	}
}

func TestProbeReportsAnUnhealthyServer(t *testing.T) {
	setup(t)

	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, `{"error":{"message":"loading model"}}`)
	})

	h := c.Probe(context.Background())
	if h.OK {
		t.Error("Probe reported OK for a 503")
	}
	if !errors.Is(h.Err, ErrUnreachable) {
		t.Errorf("Err = %v, want ErrUnreachable", h.Err)
	}
	if h.Checked.IsZero() {
		t.Error("Checked was not set on a failed probe, so the failure will not be cached")
	}
}

// The semaphore exists so a queue of background work cannot starve the UI, and
// Priority exists so the latency-critical path never sits in that queue.
func TestSemaphoreLimitsInFlightAndPriorityBypasses(t *testing.T) {
	setup(t)

	entered := make(chan string, 8)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- r.URL.Path
		<-release
		io.WriteString(w, `{"content":"ok","stop":true}`)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	c := New(Config{Endpoint: endpointOf(srv), MaxInFlight: 1, Timeout: 10 * time.Second})

	go c.Complete(context.Background(), Request{Prompt: "first"})
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("the first request never reached the server")
	}

	// A second ordinary request waits for the slot.
	go c.Complete(context.Background(), Request{Prompt: "second"})
	select {
	case p := <-entered:
		t.Fatalf("a queued request reached %s while the only slot was taken", p)
	case <-time.After(150 * time.Millisecond):
	}

	// A priority request skips the queue.
	go c.Complete(context.Background(), Request{Prompt: "urgent", Priority: true})
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("a Priority request was held in the queue")
	}

	// So does /infill, without asking.
	go c.Complete(context.Background(), Request{Prompt: "pre", Suffix: "post"})
	select {
	case p := <-entered:
		if p != "/infill" {
			t.Errorf("expected the infill request, got %s", p)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("an /infill request was held in the queue")
	}
}

func TestBuildBody(t *testing.T) {
	tests := []struct {
		name    string
		req     Request
		stream  bool
		want    map[string]any
		absent  []string
		wantSub map[string]string
	}{
		{
			name: "chat with a system prompt",
			req:  Request{System: "be terse", Messages: []Message{{Role: RoleUser, Content: "hi"}}, Model: "m", MaxTokens: 220},
			want: map[string]any{"stream": false, "model": "m", "max_tokens": 220, "temperature": 0.0},
		},
		{
			name:   "streaming asks for usage",
			req:    Request{Messages: []Message{{Role: RoleUser, Content: "hi"}}},
			stream: true,
			want:   map[string]any{"stream": true},
		},
		{
			name:   "completions carries the prompt",
			req:    Request{Prompt: "2+2=", Temperature: 0.2, TopP: 0.9, Seed: 7, Stop: []string{"\n\n\n"}},
			want:   map[string]any{"prompt": "2+2=", "temperature": 0.2, "top_p": 0.9, "seed": 7},
			absent: []string{"input_prefix", "messages"},
		},
		{
			name:   "infill splits around the cursor",
			req:    Request{Prompt: "func add(", Suffix: ") int {", MaxTokens: 32},
			want:   map[string]any{"input_prefix": "func add(", "input_suffix": ") int {", "n_predict": 32},
			absent: []string{"max_tokens", "prompt", "messages", "stream_options"},
		},
		{
			name:   "grammar is sent when set",
			req:    Request{Prompt: "x", Grammar: `root ::= "a"`},
			want:   map[string]any{"grammar": `root ::= "a"`},
			absent: []string{"response_format"},
		},
		{
			name:   "zero values are omitted",
			req:    Request{Prompt: "x"},
			absent: []string{"grammar", "seed", "top_p", "stop", "max_tokens", "model", "response_format"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := buildBody(routeFor(tt.req), tt.req, tt.stream)

			// Round-trip through JSON: what matters is what the server sees.
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			for k, want := range tt.want {
				gotVal, ok := got[k]
				if !ok {
					t.Errorf("%q missing from the body", k)
					continue
				}
				if !sameJSON(gotVal, want) {
					t.Errorf("%q = %#v, want %#v", k, gotVal, want)
				}
			}
			for _, k := range tt.absent {
				if _, ok := got[k]; ok {
					t.Errorf("%q should not be in the body, got %#v", k, got[k])
				}
			}
			// Temperature is always sent, so two callers asking for the same
			// thing get the same sampling.
			if _, ok := got["temperature"]; !ok {
				t.Error("temperature must always be sent")
			}
			if tt.stream {
				if _, ok := got["stream_options"]; !ok && routeFor(tt.req) != routeInfill {
					t.Error("a streamed OpenAI request must ask for usage")
				}
			}
		})
	}
}

func TestChatBodyPrependsSystem(t *testing.T) {
	setup(t)

	body := chatBody(Request{
		System:   "be terse",
		Messages: []Message{{Role: RoleUser, Content: "hi"}, {Role: RoleAssistant, Content: "yo"}},
	}, false)

	msgs, ok := body["messages"].([]Message)
	if !ok {
		t.Fatalf("messages = %#v, want []Message", body["messages"])
	}
	want := []Message{
		{Role: RoleSystem, Content: "be terse"},
		{Role: RoleUser, Content: "hi"},
		{Role: RoleAssistant, Content: "yo"},
	}
	if !reflect.DeepEqual(msgs, want) {
		t.Errorf("messages = %#v, want %#v", msgs, want)
	}
}

func TestCompletionBodyKeepsSystem(t *testing.T) {
	body := completionBody(Request{System: "be terse", Prompt: "hi"}, false)
	if got, want := body["prompt"], "be terse\n\nhi"; got != want {
		t.Errorf("prompt = %q, want %q — a set System must not be silently dropped", got, want)
	}
}

func TestApplyFormat(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		wantType string
	}{
		{"empty", "", ""},
		{"json", "json", "json_object"},
		{"JSON uppercase", "JSON", "json_object"},
		{"schema", `{"type":"object","properties":{}}`, "json_schema"},
		{"nonsense", "yaml", ""},
	}

	for _, tt := range tests {
		b := map[string]any{}
		applyFormat(b, tt.format)

		rf, ok := b["response_format"].(map[string]any)
		if tt.wantType == "" {
			if ok {
				t.Errorf("%s: response_format = %#v, want none", tt.name, b["response_format"])
			}
			continue
		}
		if !ok {
			t.Errorf("%s: response_format missing", tt.name)
			continue
		}
		if rf["type"] != tt.wantType {
			t.Errorf("%s: type = %v, want %q", tt.name, rf["type"], tt.wantType)
		}
	}
}

func TestErrMessage(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"openai envelope", `{"error":{"message":"boom","type":"x"}}`, "boom"},
		{"plain error string", `{"error":"boom"}`, "boom"},
		{"message field", `{"message":"boom"}`, "boom"},
		{"html collapses to one line", "<h1>502</h1>\n<p>Bad\n  Gateway</p>", "<h1>502</h1> <p>Bad Gateway</p>"},
		{"empty", "", ""},
		{"long body is truncated", strings.Repeat("z", 400), strings.Repeat("z", 200) + "…"},
	}

	for _, tt := range tests {
		if got := errMessage([]byte(tt.body)); got != tt.want {
			t.Errorf("%s: errMessage = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// sameJSON compares a decoded JSON value against a Go literal, since numbers
// come back as float64.
func sameJSON(got, want any) bool {
	switch w := want.(type) {
	case int:
		g, ok := got.(float64)
		return ok && g == float64(w)
	case float64:
		g, ok := got.(float64)
		return ok && g == w
	default:
		return reflect.DeepEqual(got, want)
	}
}
