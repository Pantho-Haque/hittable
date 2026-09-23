package llm

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestFakeSatisfiesTheClientInterface(t *testing.T) {
	setup(t)

	var c Client = NewFake("hello")
	resp, err := c.Complete(context.Background(), Request{Prompt: "x"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "hello" {
		t.Errorf("Text = %q, want %q", resp.Text, "hello")
	}
}

func TestFakeStreamsChunks(t *testing.T) {
	setup(t)

	f := NewFakeStream("fix", "(git): ", "keep the error")
	var got collector

	resp, err := f.Stream(context.Background(), Request{Prompt: "x"}, got.fn)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	want := []string{"fix", "(git): ", "keep the error"}
	if !reflect.DeepEqual(got.chunks, want) {
		t.Errorf("chunks = %q, want %q", got.chunks, want)
	}
	if resp.Text != "fix(git): keep the error" {
		t.Errorf("Text = %q, want the joined chunks", resp.Text)
	}
	if got.done != 1 {
		t.Errorf("Done chunks = %d, want 1", got.done)
	}
}

func TestFakeConsumesTurnsInOrder(t *testing.T) {
	setup(t)

	f := &Fake{Default: FakeTurn{Text: "default"}}
	f.Push(FakeTurn{Text: "first"}).Push(FakeTurn{Text: "second", Err: errors.New("boom")})

	tests := []struct {
		wantText string
		wantErr  bool
	}{
		{"first", false},
		{"", true},
		{"default", false},
		{"default", false},
	}

	for i, tt := range tests {
		resp, err := f.Complete(context.Background(), Request{Prompt: "x"})
		if (err != nil) != tt.wantErr {
			t.Fatalf("call %d: err = %v, wantErr = %v", i, err, tt.wantErr)
		}
		if tt.wantErr {
			continue
		}
		if resp.Text != tt.wantText {
			t.Errorf("call %d: Text = %q, want %q", i, resp.Text, tt.wantText)
		}
	}

	if got := len(f.Calls()); got != len(tests) {
		t.Errorf("recorded %d calls, want %d", got, len(tests))
	}
}

// The Fake has to model the one behaviour that matters most in the real
// client: an error after some text still returns the text.
func TestFakeKeepsPartialTextOnError(t *testing.T) {
	setup(t)

	f := &Fake{Default: FakeTurn{Chunks: []string{"half "}, Err: errors.New("crashed")}}

	resp, err := f.Stream(context.Background(), Request{Prompt: "x"}, nil)
	if err == nil {
		t.Fatal("Stream returned no error")
	}
	if resp == nil || resp.Text != "half " {
		t.Fatalf("partial text lost: %+v", resp)
	}
}

func TestFakeHonoursCancellation(t *testing.T) {
	setup(t)

	f := &Fake{Default: FakeTurn{Chunks: []string{"a", "b", "c"}, Delay: 50 * time.Millisecond}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var got collector
	resp, err := f.Stream(ctx, Request{Prompt: "x"}, func(ch Chunk) {
		got.fn(ch)
		cancel()
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if resp.Text != "a" {
		t.Errorf("partial Text = %q, want %q", resp.Text, "a")
	}
}

func TestFakeHonoursDeadline(t *testing.T) {
	setup(t)

	f := &Fake{Default: FakeTurn{Text: "too late", Delay: time.Second}}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if _, err := f.Complete(ctx, Request{Prompt: "x"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestFakeRecordsRequests(t *testing.T) {
	setup(t)

	f := NewFake("ok")
	f.Complete(context.Background(), Request{Prompt: "one", Temperature: 0.2})
	f.Complete(context.Background(), Request{Prompt: "two", Grammar: "root ::= \"a\""})

	last, ok := f.LastRequest()
	if !ok {
		t.Fatal("LastRequest reported nothing")
	}
	if last.Prompt != "two" || last.Grammar == "" {
		t.Errorf("LastRequest = %+v, want the second request", last)
	}
	if got := f.Calls(); len(got) != 2 || got[0].Prompt != "one" {
		t.Errorf("Calls = %+v, want both requests in order", got)
	}
}

func TestFakeProbeCountsCalls(t *testing.T) {
	setup(t)

	f := &Fake{HealthResult: Health{OK: true, Model: "fake"}}
	if f.Probes() != 0 {
		t.Fatalf("Probes = %d before any call, want 0", f.Probes())
	}

	h := f.Probe(context.Background())
	if !h.OK || h.Model != "fake" {
		t.Errorf("Probe = %+v, want the scripted health", h)
	}
	if h.Checked.IsZero() {
		t.Error("Probe left Checked zero, so a caller cannot tell it ever ran")
	}
	if f.Probes() != 1 {
		t.Errorf("Probes = %d, want 1", f.Probes())
	}
}
