package llm

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Fake is a scripted Client with no HTTP in it at all. It is deliberately not a
// _test.go file: the packages that consume this seam — commit drafting, the git
// panel, the response viewer — all need a client they can script, and a test
// helper that only its own package can import is no help to them.
//
// The zero Fake returns empty text and no error. Set Default for one canned
// answer, or Push turns for a call-by-call script; turns are consumed in order
// and Default takes over once they run out.
type Fake struct {
	// Turns are consumed one per Complete/Stream call.
	Turns []FakeTurn
	// Default answers every call once Turns is exhausted.
	Default FakeTurn
	// HealthResult is what Probe reports.
	HealthResult Health

	mu     sync.Mutex
	calls  []Request
	next   int
	probes int
}

// FakeTurn is one scripted answer.
type FakeTurn struct {
	// Chunks are streamed in order. Complete joins them.
	Chunks []string
	// Text overrides the joined chunks as the final text. When Chunks is empty
	// Stream emits Text as a single chunk.
	Text string
	// Err is returned after the chunks have been delivered, so a test can
	// script a stream that fails half way and still assert on partial text.
	Err error

	StopReason       string
	PromptTokens     int
	CompletionTokens int

	// Delay is applied before each chunk, and honours the context — which is
	// how a test drives a caller's timeout path without a real server.
	Delay time.Duration
}

// NewFake returns a Fake that always answers with text in one chunk.
func NewFake(text string) *Fake {
	return &Fake{Default: FakeTurn{Text: text}}
}

// NewFakeStream returns a Fake that always streams the given chunks.
func NewFakeStream(chunks ...string) *Fake {
	return &Fake{Default: FakeTurn{Chunks: chunks}}
}

// Push appends a scripted turn and returns the Fake, so turns can be chained.
func (f *Fake) Push(t FakeTurn) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Turns = append(f.Turns, t)
	return f
}

// Complete implements Client.
func (f *Fake) Complete(ctx context.Context, req Request) (*Response, error) {
	turn := f.take(req)

	if err := sleep(ctx, turn.Delay); err != nil {
		return nil, err
	}
	if turn.Err != nil {
		return nil, turn.Err
	}
	return turn.response(turn.fullText()), nil
}

// Stream implements Client.
func (f *Fake) Stream(ctx context.Context, req Request, onChunk func(Chunk)) (*Response, error) {
	if onChunk == nil {
		onChunk = func(Chunk) {}
	}
	turn := f.take(req)

	chunks := turn.Chunks
	if len(chunks) == 0 && turn.Text != "" {
		chunks = []string{turn.Text}
	}

	var sb strings.Builder
	for _, ch := range chunks {
		if err := sleep(ctx, turn.Delay); err != nil {
			return turn.response(sb.String()), err
		}
		sb.WriteString(ch)
		onChunk(Chunk{Text: ch})
	}
	if turn.Err != nil {
		return turn.response(sb.String()), turn.Err
	}

	text := sb.String()
	if turn.Text != "" {
		text = turn.Text
	}
	onChunk(Chunk{Done: true})
	return turn.response(text), nil
}

// Probe implements Client. It never performs I/O, and counts its calls so a
// test can assert that startup probed nothing.
func (f *Fake) Probe(ctx context.Context) Health {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.probes++

	h := f.HealthResult
	if h.Checked.IsZero() {
		h.Checked = time.Now()
	}
	return h
}

// Calls returns a copy of every request the Fake was given, in order.
func (f *Fake) Calls() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Request(nil), f.calls...)
}

// LastRequest returns the most recent request, if there was one.
func (f *Fake) LastRequest() (Request, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return Request{}, false
	}
	return f.calls[len(f.calls)-1], true
}

// Probes is how many times Probe was called.
func (f *Fake) Probes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.probes
}

// take records the request and returns the turn that answers it.
func (f *Fake) take(req Request) FakeTurn {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, req)

	if f.next < len(f.Turns) {
		t := f.Turns[f.next]
		f.next++
		return t
	}
	return f.Default
}

func (t FakeTurn) fullText() string {
	if t.Text != "" {
		return t.Text
	}
	return strings.Join(t.Chunks, "")
}

func (t FakeTurn) response(text string) *Response {
	return &Response{
		Text:             text,
		StopReason:       t.StopReason,
		PromptTokens:     t.PromptTokens,
		CompletionTokens: t.CompletionTokens,
	}
}

// sleep waits for d, or returns the context's error if that comes first. A zero
// delay still checks the context, so an already-cancelled caller never gets a
// scripted answer.
func sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var _ Client = (*Fake)(nil)
