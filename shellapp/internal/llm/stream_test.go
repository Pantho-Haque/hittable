package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestStreamSSE(t *testing.T) {
	setup(t)

	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		fl := newFlusher(t, w)
		fl.write(": keep-alive\n\n")
		fl.write(`data: {"choices":[{"delta":{"content":"fix"}}]}` + "\n\n")
		fl.write("\n")
		fl.write(`data: {"choices":[{"delta":{"content":"(git): "}}]}` + "\n\n")
		fl.write(`data: {"choices":[{"delta":{"content":"stop swallowing the diff error"},"finish_reason":"stop"}]}` + "\n\n")
		fl.write(`data: {"choices":[],"usage":{"prompt_tokens":412,"completion_tokens":11}}` + "\n\n")
		fl.write("data: [DONE]\n\n")
	})

	var got collector
	resp, err := c.Stream(context.Background(), Request{Messages: []Message{{Role: RoleUser, Content: "hi"}}}, got.fn)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	wantChunks := []string{"fix", "(git): ", "stop swallowing the diff error"}
	if !reflect.DeepEqual(got.chunks, wantChunks) {
		t.Errorf("chunks = %q, want %q", got.chunks, wantChunks)
	}
	if want := "fix(git): stop swallowing the diff error"; resp.Text != want {
		t.Errorf("Text = %q, want %q", resp.Text, want)
	}
	if got.done != 1 {
		t.Errorf("Done chunks = %d, want exactly 1", got.done)
	}
	if resp.PromptTokens != 412 || resp.CompletionTokens != 11 {
		t.Errorf("tokens = %d/%d, want 412/11", resp.PromptTokens, resp.CompletionTokens)
	}
	if resp.StopReason != "stop" {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, "stop")
	}
	if resp.TTFT <= 0 || resp.Total <= 0 {
		t.Errorf("timings not measured: TTFT=%v Total=%v", resp.TTFT, resp.Total)
	}
}

func TestStreamNDJSON(t *testing.T) {
	setup(t)

	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		fl := newFlusher(t, w)
		fl.write(`{"content":"func ","stop":false}` + "\n")
		fl.write(`{"content":"main","stop":false}` + "\n")
		fl.write(`{"content":"() {","stop":true,"stop_type":"eos","tokens_evaluated":88,"tokens_predicted":4}` + "\n")
		// Anything after the terminal frame must be ignored, not appended.
		fl.write(`{"content":"NEVER","stop":false}` + "\n")
	})

	var got collector
	resp, err := c.Stream(context.Background(), Request{Prompt: "write "}, got.fn)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	wantChunks := []string{"func ", "main", "() {"}
	if !reflect.DeepEqual(got.chunks, wantChunks) {
		t.Errorf("chunks = %q, want %q", got.chunks, wantChunks)
	}
	if want := "func main() {"; resp.Text != want {
		t.Errorf("Text = %q, want %q", resp.Text, want)
	}
	if resp.PromptTokens != 88 || resp.CompletionTokens != 4 {
		t.Errorf("tokens = %d/%d, want 88/4", resp.PromptTokens, resp.CompletionTokens)
	}
	if resp.StopReason != "eos" {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, "eos")
	}
}

// A single SSE frame arriving in two writes is the exact failure this repo
// already had once, in the mouse-report reader (v2.8.3): a reader that treats
// "what one read returned" as "one whole record" corrupts the tail. The
// scanner must join the halves, so the chunk arrives once and intact.
func TestStreamFrameSplitAcrossWrites(t *testing.T) {
	setup(t)

	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		fl := newFlusher(t, w)
		frame := `data: {"choices":[{"delta":{"content":"half a frame is not a frame"}}]}` + "\n\n"
		cut := len(frame) / 2

		fl.write(frame[:cut])
		fl.write(frame[cut:])
		fl.write("data: [DONE]\n\n")
	})

	var got collector
	resp, err := c.Stream(context.Background(), Request{Prompt: "x"}, got.fn)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if want := []string{"half a frame is not a frame"}; !reflect.DeepEqual(got.chunks, want) {
		t.Errorf("chunks = %q, want %q", got.chunks, want)
	}
	if want := "half a frame is not a frame"; resp.Text != want {
		t.Errorf("Text = %q, want %q", resp.Text, want)
	}
}

// bufio.Scanner refuses a line longer than 64KB by default, and the symptom is
// a response that silently stops early. The explicit Buffer call is what makes
// a megabyte frame work; this test fails without it.
func TestStreamLineOverAMegabyte(t *testing.T) {
	setup(t)

	huge := strings.Repeat("x", 1<<20)
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		fl := newFlusher(t, w)
		fl.write(`data: {"choices":[{"delta":{"content":"` + huge + `"}}]}` + "\n\n")
		fl.write("data: [DONE]\n\n")
	})

	var got collector
	resp, err := c.Stream(context.Background(), Request{Prompt: "x"}, got.fn)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(resp.Text) != len(huge) {
		t.Errorf("Text length = %d, want %d", len(resp.Text), len(huge))
	}
	if len(got.chunks) != 1 {
		t.Errorf("chunks = %d, want 1", len(got.chunks))
	}
}

// Cancelling mid-stream must be distinguishable from the model failing, and
// must not throw away what the user is already reading.
func TestStreamCancelKeepsPartialText(t *testing.T) {
	setup(t)

	// The handler holds the stream open, so it needs the same release valve as
	// silentServer: drain the body, and let the test end it. The cleanup that
	// closes stop is registered after newClient registered srv.Close, so it
	// runs first and Close never waits on a live handler.
	stop := make(chan struct{})
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)

		fl := newFlusher(t, w)
		fl.write(`data: {"choices":[{"delta":{"content":"kept"}}]}` + "\n\n")
		// Hold the stream open until the client goes away, so the scanner is
		// blocked on a read when the cancel lands.
		select {
		case <-stop:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(stop) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var got collector
	resp, err := c.Stream(ctx, Request{Prompt: "x"}, func(ch Chunk) {
		got.fn(ch)
		cancel()
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if errors.Is(err, ErrTimeout) || errors.Is(err, ErrUnreachable) {
		t.Errorf("cancellation was reported as a failure: %v", err)
	}
	if resp == nil {
		t.Fatal("Stream returned a nil response on cancel; the partial text is lost")
	}
	if resp.Text != "kept" {
		t.Errorf("partial Text = %q, want %q", resp.Text, "kept")
	}
	if got.done != 0 {
		t.Errorf("Done chunk delivered %d times on a cancelled stream, want 0", got.done)
	}
}

func TestStreamSkipsUnparseableFrames(t *testing.T) {
	setup(t)

	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		fl := newFlusher(t, w)
		fl.write(`data: {"choices":[{"delta":{"content":"a"}}]}` + "\n\n")
		fl.write("data: {not json at all\n\n")
		fl.write("event: ping\n\n")
		fl.write(`data: {"choices":[{"delta":{"content":"b"}}]}` + "\n\n")
		fl.write("data: [DONE]\n\n")
	})

	var got collector
	resp, err := c.Stream(context.Background(), Request{Prompt: "x"}, got.fn)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if resp.Text != "ab" {
		t.Errorf("Text = %q, want %q", resp.Text, "ab")
	}
}

func TestStreamServerErrorFrame(t *testing.T) {
	setup(t)

	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		fl := newFlusher(t, w)
		fl.write(`data: {"choices":[{"delta":{"content":"partial"}}]}` + "\n\n")
		fl.write(`data: {"error":{"message":"context shift is disabled","type":"server"}}` + "\n\n")
	})

	var got collector
	resp, err := c.Stream(context.Background(), Request{Prompt: "x"}, got.fn)
	if err == nil {
		t.Fatal("Stream returned no error for an error frame")
	}
	if !strings.Contains(err.Error(), "context shift is disabled") {
		t.Errorf("err = %v, want the server's message", err)
	}
	if resp.Text != "partial" {
		t.Errorf("partial Text = %q, want %q", resp.Text, "partial")
	}
}

func TestDecodeLine(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		wantPayload  string
		wantOK       bool
		wantTerminal bool
	}{
		{"blank", "", "", false, false},
		{"whitespace", "   ", "", false, false},
		{"sse comment", ": ping", "", false, false},
		{"sse event field", "event: message", "", false, false},
		{"sse id field", "id: 7", "", false, false},
		{"sse data", `data: {"a":1}`, `{"a":1}`, true, false},
		{"sse data no space", `data:{"a":1}`, `{"a":1}`, true, false},
		{"sse empty data", "data:", "", false, false},
		{"sse done", "data: [DONE]", "", false, true},
		{"ndjson", `{"content":"x","stop":false}`, `{"content":"x","stop":false}`, true, false},
		{"garbage", "llama_perf_context_print: load time = 1ms", "", false, false},
	}

	for _, tt := range tests {
		payload, ok, terminal := decodeLine(tt.line)
		if payload != tt.wantPayload || ok != tt.wantOK || terminal != tt.wantTerminal {
			t.Errorf("decodeLine(%q) = (%q, %v, %v), want (%q, %v, %v)",
				tt.line, payload, ok, terminal, tt.wantPayload, tt.wantOK, tt.wantTerminal)
		}
	}
}

func TestFrameTerminal(t *testing.T) {
	tests := []struct {
		name string
		json string
		want bool
	}{
		{"stop true", `{"stop":true}`, true},
		{"stop false", `{"stop":false}`, false},
		{"done true", `{"done":true}`, true},
		{"done false", `{"done":false}`, false},
		{"neither", `{"content":"x"}`, false},
		{"openai delta", `{"choices":[{"delta":{"content":"x"}}]}`, false},
	}

	for _, tt := range tests {
		var f frame
		if err := unmarshalFrame(tt.json, &f); err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got := f.terminal(); got != tt.want {
			t.Errorf("%s: terminal() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestFrameText(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"chat delta", `{"choices":[{"delta":{"content":"a"}}]}`, "a"},
		{"chat message", `{"choices":[{"message":{"content":"b"}}]}`, "b"},
		{"completion text", `{"choices":[{"text":"c"}]}`, "c"},
		{"llama content", `{"content":"d"}`, "d"},
		{"usage only", `{"choices":[],"usage":{"prompt_tokens":1}}`, ""},
	}

	for _, tt := range tests {
		var f frame
		if err := unmarshalFrame(tt.json, &f); err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got := f.text(); got != tt.want {
			t.Errorf("%s: text() = %q, want %q", tt.name, got, tt.want)
		}
	}
}
