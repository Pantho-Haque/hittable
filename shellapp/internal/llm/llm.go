// Package llm is a stdlib HTTP client for a local OpenAI-compatible inference
// server (llama.cpp's llama-server). One Request type covers all three routes —
// chat, raw completion and fill-in-the-middle — because they differ only in
// which field is set, and every caller wants identical cancellation, timeout
// and error semantics.
//
// Non-goals: this package knows nothing about commits, git, prompts, models on
// disk, or how the server got started. It does not download, spawn, supervise
// or configure anything, it reads no files and no environment variables, and it
// imports nothing else from this app — that is what makes it reusable. The
// address of the server arrives through Config.Endpoint, a function, so the
// supervisor can pick a port at spawn time and the first request that needs a
// server is what triggers one.
package llm

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// Role is the author of a chat message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one turn of a chat conversation.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// Request is one generation. Which fields are set decides the route:
//
//	Suffix != ""                 -> /infill            (fill-in-the-middle)
//	Prompt != "" (no Suffix)     -> /v1/completions    (raw completion)
//	otherwise                    -> /v1/chat/completions
//
// Temperature is always sent, so the zero value means greedy decoding rather
// than "whatever the server defaults to" — two callers asking for the same
// thing must get the same sampling.
type Request struct {
	Messages []Message // chat turns
	Prompt   string    // raw prompt, or the prefix when Suffix is set
	Suffix   string    // text after the cursor; its presence selects /infill
	System   string    // convenience: prepended as a system Message on the chat route

	Model       string
	MaxTokens   int
	Temperature float64
	TopP        float64 // sent when > 0
	Stop        []string
	Seed        int    // sent when non-zero
	Grammar     string // GBNF, honoured by llama.cpp, omitted when empty
	Format      string // "json", or a JSON schema object

	// Priority skips the in-flight semaphore. Inline completion has an 800ms
	// budget and is useless queued behind a 200-token stream.
	Priority bool
}

// Chunk is one streamed delta. The final callback of a stream has Done set and
// no text.
type Chunk struct {
	Text string
	Done bool
}

// Response is the result of a generation. Stream fills Text with everything it
// received, including when it returns an error — a cancelled stream keeps what
// already arrived so the UI does not blank out what the user was reading.
type Response struct {
	Text       string
	StopReason string

	PromptTokens     int
	CompletionTokens int

	TTFT  time.Duration // time to the first byte of generated text
	Total time.Duration
}

// Health is the result of Probe.
type Health struct {
	OK       bool
	Endpoint string
	Model    string
	Err      error
	Checked  time.Time
	Latency  time.Duration
}

// Client is the seam every consumer depends on. Fake implements it with no
// HTTP at all.
type Client interface {
	Complete(ctx context.Context, req Request) (*Response, error)
	Stream(ctx context.Context, req Request, onChunk func(Chunk)) (*Response, error)
	Probe(ctx context.Context) Health
}

// Errors callers are expected to branch on. Transport and protocol failures
// wrap one of these; a cancelled context is reported as context.Canceled, never
// as one of these, so "the user pressed esc" stays distinguishable from "the
// model failed".
var (
	// ErrDisabled means there is no endpoint to talk to and no intention of
	// getting one: the client was built without an Endpoint function, or the
	// function returned an empty address.
	ErrDisabled = errors.New("llm: disabled")
	// ErrUnreachable means the address is known but nothing answered.
	ErrUnreachable = errors.New("llm: server unreachable")
	// ErrModelMissing means the server answered but does not have the model.
	ErrModelMissing = errors.New("llm: model not available")
	// ErrTimeout means this client's own deadline fired, as opposed to the
	// caller's context being cancelled.
	ErrTimeout = errors.New("llm: timed out")
)

// Config configures an HTTPClient.
type Config struct {
	// Endpoint returns the base URL of the server, e.g. "http://127.0.0.1:8081".
	// It is a function, not a string, because the port is chosen at spawn time
	// and the first call may be what spawns the server. A nil Endpoint, or one
	// that returns an empty string, makes every request fail with ErrDisabled.
	Endpoint func(context.Context) (string, error)

	// Timeout bounds a request that arrives without its own deadline.
	// Default 60s.
	Timeout time.Duration
	// FastTimeout bounds the /infill route, which is on the typing path.
	// Default 800ms.
	FastTimeout time.Duration
	// MaxInFlight is the number of concurrent non-priority requests.
	// Default 2.
	MaxInFlight int

	// HTTPClient overrides the shared client. Tests that need to observe the
	// transport set it; nothing else should.
	HTTPClient *http.Client
	// Now overrides the clock, for tests of the probe cache.
	Now func() time.Time
}

// Defaults for Config.
const (
	DefaultTimeout     = 60 * time.Second
	DefaultFastTimeout = 800 * time.Millisecond
	DefaultMaxInFlight = 2

	// probeTTL is how long a Probe result is reused. A health check is cheap
	// but not free, and the UI asks on every frame it repaints a status line.
	probeTTL = 60 * time.Second
	// probeTimeout is deliberately independent of Config.Timeout: "is it up"
	// must answer fast even when generation is allowed a minute.
	probeTimeout = 300 * time.Millisecond
)

// sharedTransport is process-wide so the TCP handshake never lands inside
// time-to-first-token. Proxy is nil on purpose: the server is on loopback and a
// corporate HTTP_PROXY must not swallow the traffic.
var sharedTransport = &http.Transport{
	Proxy: nil,
	DialContext: (&net.Dialer{
		Timeout:   2 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	MaxIdleConns:          8,
	MaxIdleConnsPerHost:   4,
	IdleConnTimeout:       5 * time.Minute,
	TLSHandshakeTimeout:   5 * time.Second,
	ExpectContinueTimeout: time.Second,
	ForceAttemptHTTP2:     false,
}

// New builds an HTTPClient, filling in the defaults for anything left zero.
func New(cfg Config) *HTTPClient {
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.FastTimeout <= 0 {
		cfg.FastTimeout = DefaultFastTimeout
	}
	if cfg.MaxInFlight <= 0 {
		cfg.MaxInFlight = DefaultMaxInFlight
	}
	if cfg.HTTPClient == nil {
		// No client-level Timeout: a stream is bounded by its context, and a
		// http.Client.Timeout would cut a long generation off mid-sentence.
		cfg.HTTPClient = &http.Client{Transport: sharedTransport}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &HTTPClient{
		cfg: cfg,
		sem: make(chan struct{}, cfg.MaxInFlight),
	}
}

var _ Client = (*HTTPClient)(nil)
