package llm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// maxStreamLine is the largest single line the stream reader will accept.
// bufio.Scanner defaults to 64KB and reports a longer line as an error, which
// in practice reads as a silently truncated response — and one SSE frame
// carrying a whole tool call or a re-sent prompt does exceed 64KB.
const maxStreamLine = 4 << 20

// frame is every wire shape this client understands, in one struct: the
// OpenAI-compatible chat delta and completion choice, and llama.cpp's own
// top-level content. Which fields are populated tells you which one arrived.
type frame struct {
	// llama.cpp /completion and /infill
	Content         string `json:"content"`
	Stop            *bool  `json:"stop"`
	Done            *bool  `json:"done"`
	StopType        string `json:"stop_type"`
	TokensPredicted int    `json:"tokens_predicted"`
	TokensEvaluated int    `json:"tokens_evaluated"`

	// OpenAI-compatible
	Choices []struct {
		Text  string `json:"text"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`

	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// text is the generated text this frame carries, whichever shape it is in.
func (f *frame) text() string {
	if len(f.Choices) > 0 {
		ch := f.Choices[0]
		switch {
		case ch.Delta.Content != "":
			return ch.Delta.Content
		case ch.Text != "":
			return ch.Text
		case ch.Message.Content != "":
			return ch.Message.Content
		}
		return ""
	}
	return f.Content
}

// terminal reports the end-of-generation markers llama.cpp puts on its last
// frame. An OpenAI-compatible stream has no such field and ends at [DONE].
func (f *frame) terminal() bool {
	return (f.Stop != nil && *f.Stop) || (f.Done != nil && *f.Done)
}

// apply folds whatever metadata the frame carries into the response. Token
// counts arrive on the last frame only, under one of two names.
func (f *frame) apply(resp *Response) {
	if f.Usage.PromptTokens > 0 {
		resp.PromptTokens = f.Usage.PromptTokens
	}
	if f.Usage.CompletionTokens > 0 {
		resp.CompletionTokens = f.Usage.CompletionTokens
	}
	if f.TokensEvaluated > 0 {
		resp.PromptTokens = f.TokensEvaluated
	}
	if f.TokensPredicted > 0 {
		resp.CompletionTokens = f.TokensPredicted
	}
	if r := f.stopReason(); r != "" {
		resp.StopReason = r
	}
}

func (f *frame) stopReason() string {
	if len(f.Choices) > 0 && f.Choices[0].FinishReason != "" {
		return f.Choices[0].FinishReason
	}
	if f.StopType != "" {
		return f.StopType
	}
	if f.terminal() {
		return "stop"
	}
	return ""
}

// readStream consumes a streamed body, whichever of the two framings the server
// chose, and calls onChunk for each delta. One Scanner serves both: the framing
// only decides how a line becomes a JSON payload.
func readStream(body io.Reader, onChunk func(Chunk), resp *Response, start time.Time, now func() time.Time) error {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64<<10), maxStreamLine)

	for sc.Scan() {
		payload, ok, terminal := decodeLine(sc.Text())
		if terminal && !ok {
			return nil
		}
		if !ok {
			continue
		}

		var f frame
		if err := json.Unmarshal([]byte(payload), &f); err != nil {
			// A frame we cannot parse is not worth killing a generation the
			// user is already reading; the next one usually parses.
			continue
		}
		if f.Error != nil {
			return fmt.Errorf("llm: server error during stream: %s", f.Error.Message)
		}

		if txt := f.text(); txt != "" {
			if resp.TTFT == 0 {
				resp.TTFT = now().Sub(start)
			}
			resp.Text += txt
			onChunk(Chunk{Text: txt})
		}
		f.apply(resp)

		if f.terminal() {
			return nil
		}
	}
	return sc.Err()
}

// decodeLine picks the framing for one line and returns its JSON payload. ok is
// false for a line that carries nothing; terminal is the framing's own
// end-of-stream marker, as opposed to the one inside a frame.
func decodeLine(line string) (payload string, ok, terminal bool) {
	t := strings.TrimSpace(line)
	switch {
	case t == "":
		return "", false, false
	case strings.HasPrefix(t, "{"):
		return ndjsonLine(t)
	default:
		return sseLine(t)
	}
}

// sseLine handles text/event-stream: blank lines and ":" comments (keep-alives)
// carry nothing, only "data:" lines do, and "data: [DONE]" ends the stream.
// Other SSE fields — event, id, retry — are metadata we have no use for.
func sseLine(line string) (payload string, ok, terminal bool) {
	if strings.HasPrefix(line, ":") {
		return "", false, false
	}
	data, found := strings.CutPrefix(line, "data:")
	if !found {
		return "", false, false
	}
	data = strings.TrimSpace(data)
	switch {
	case data == "":
		return "", false, false
	case data == "[DONE]":
		return "", false, true
	}
	return data, true, false
}

// ndjsonLine handles newline-delimited JSON: the whole line is the object, and
// the end of the stream is a field inside it ("stop" or "done"), which the
// caller checks once the object is decoded.
func ndjsonLine(line string) (payload string, ok, terminal bool) {
	return line, true, false
}

// readWhole consumes a non-streamed body.
func readWhole(body io.Reader, resp *Response) error {
	data, err := io.ReadAll(io.LimitReader(body, maxResponseBytes))
	if err != nil {
		return err
	}

	var f frame
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("llm: unreadable response: %w", err)
	}
	if f.Error != nil {
		return fmt.Errorf("llm: server error: %s", f.Error.Message)
	}

	resp.Text = f.text()
	f.apply(resp)
	if resp.StopReason == "" {
		resp.StopReason = "stop"
	}
	return nil
}
