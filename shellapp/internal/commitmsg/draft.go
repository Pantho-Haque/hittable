package commitmsg

import (
	"context"
	"strings"

	"github.com/hittable/shellapp/internal/llm"
)

// Drafter rewrites the heuristic draft into prose. The git panel declares this
// interface itself so it depends on no inference package; ClientDrafter is the
// implementation that does.
type Drafter interface {
	DraftStream(ctx context.Context, d *Digest, opts Options, onChunk func(string)) (Message, error)
}

// ClientDrafter adapts an llm.Client to Drafter.
type ClientDrafter struct{ Client llm.Client }

// DraftStream implements Drafter.
func (c ClientDrafter) DraftStream(ctx context.Context, d *Digest, opts Options, onChunk func(string)) (Message, error) {
	return DraftStream(ctx, c.Client, d, opts, onChunk)
}

// Draft asks the model to name the change and returns the best message
// available: the model's subject over a body written from the diff.
//
// The returned Message is always usable: every failure path ends at
// Generate's heuristic draft, so there is no such thing as an empty result.
// Read Message.Source to find out which path it took.
//
// A non-nil error means the *model* failed — no client, transport, timeout, or
// the caller's context being cancelled — and never means "the output was no
// good". Output that cannot be validated or repaired is not an error: it
// returns the heuristic message with a nil error, because the caller's editor
// needs replacing with something valid rather than being left holding the
// rejected text.
func Draft(ctx context.Context, c llm.Client, d *Digest, opts Options) (Message, error) {
	return draft(ctx, c, d, opts, nil)
}

// DraftStream is Draft with deltas delivered to onChunk as they arrive. Each
// call receives only the new text, so a caller can append into its own buffer.
//
// The one-shot re-ask that Draft performs on invalid output is skipped here:
// the user is already reading the stream, and replacing what they are reading
// with a second attempt is worse than repairing what arrived.
func DraftStream(ctx context.Context, c llm.Client, d *Digest, opts Options, onChunk func(string)) (Message, error) {
	if onChunk == nil {
		onChunk = func(string) {}
	}
	return draft(ctx, c, d, opts, onChunk)
}

func draft(ctx context.Context, c llm.Client, d *Digest, opts Options, onChunk func(string)) (Message, error) {
	fallback := Generate(d, opts)
	if c == nil || d == nil {
		return fallback, llm.ErrDisabled
	}

	req := BuildRequest(d, opts)
	prompt := promptText(req)
	sp := buildSpec(d)
	text, err := run(ctx, c, req, onChunk)
	if err != nil {
		// A cancelled context is the user pressing esc, which the caller
		// reports differently from a model that fell over. Whatever streamed
		// before the cancel is worth keeping if it happens to be a message.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return bestOf(text, fallback, prompt, sp, opts), ctxErr
		}
		return bestOf(text, fallback, prompt, sp, opts), err
	}

	if m, ok := finish(text, prompt, sp, opts); ok {
		return withBody(m, fallback), nil
	}
	// One re-ask at temperature 0, with the reason the first answer was
	// rejected. Streaming callers skip this: see DraftStream.
	if onChunk == nil {
		if m, ok := reask(ctx, c, req, prompt, sp, text, opts); ok {
			return withBody(m, fallback), nil
		}
	}
	return fallback, nil
}

// run performs the request on the streaming or the completing route and
// returns whatever text arrived, even alongside an error.
func run(ctx context.Context, c llm.Client, req llm.Request, onChunk func(string)) (string, error) {
	if onChunk != nil {
		resp, err := c.Stream(ctx, req, func(ch llm.Chunk) {
			if ch.Text != "" {
				onChunk(ch.Text)
			}
		})
		return textOf(resp), err
	}
	resp, err := c.Complete(ctx, req)
	return textOf(resp), err
}

// promptText is everything the model was shown, which is what an answer is
// checked against for echo.
func promptText(req llm.Request) string {
	var b strings.Builder
	b.WriteString(req.System)
	for _, m := range req.Messages {
		b.WriteString("\n")
		b.WriteString(m.Content)
	}
	return b.String()
}

func textOf(resp *llm.Response) string {
	if resp == nil {
		return ""
	}
	return resp.Text
}

// reask sends the rejected answer back with the reason, once, greedily. The
// model that produced malformed output usually produces valid output when
// told exactly what was wrong with it.
func reask(ctx context.Context, c llm.Client, req llm.Request, prompt string, sp spec, bad string, opts Options) (Message, bool) {
	reason := "it is not a valid commit message"
	if Echoes(bad, prompt) {
		reason = "that repeats what you were given instead of describing it"
	} else if err := Validate(bad, opts.Rules.resolved()); err != nil {
		reason = err.Error()
	}
	retry := req
	retry.Temperature = 0
	retry.Messages = append(append([]llm.Message{}, req.Messages...),
		llm.Message{Role: llm.RoleAssistant, Content: bad},
		llm.Message{Role: llm.RoleUser, Content: "That was rejected: " + reason +
			". Output only the corrected commit message, nothing else."})

	resp, err := c.Complete(ctx, retry)
	if err != nil {
		return Message{}, false
	}
	return finish(textOf(resp), prompt, sp, opts)
}

// finish validates the model's text, repairs it if it can, and reports whether
// anything usable came out.
// finish reads the model's answer: a subject line, then bullets.
//
// The subject and the body are judged separately, because they fail
// separately. A subject that cannot be validated sinks the whole answer; a
// body that cannot be grounded is simply dropped, and the caller substitutes
// the mechanical file summary. A true subject with a factual list under it is
// worth more than falling back to the heuristic for both.
func finish(text, prompt string, sp spec, opts Options) (Message, bool) {
	rules := opts.Rules.resolved()
	// Echo is checked against everything the model produced, not just the
	// lines that survive: a model that echoes the prompt usually puts a
	// plausible header on top of it, and only the tail gives it away.
	if Echoes(text, prompt) {
		return Message{}, false
	}
	lines := unwrap(text)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return Message{}, false
	}

	header := strings.TrimSpace(lines[0])
	src := SourceModel
	if Validate(header, rules) != nil {
		repaired, ok := Repair(header, rules, opts.Type)
		if !ok {
			return Message{}, false
		}
		header, src = repaired, SourceRepaired
	}
	m := parseMessage(header, rules, src)
	// The subject is committed and read first, so a name invented there costs
	// more than one in a bullet. It is held to the same standard, and failing
	// it sinks the whole answer rather than just the body.
	if !sp.empty() && ground(m.Subject, sp) != nil {
		return Message{}, false
	}

	body, kept := keepBullets(lines[1:], sp)
	m.Body = body
	if !kept || strings.TrimSpace(text) != m.String() {
		// Something was dropped, unwrapped or rewritten on the way here.
		m.Source = SourceRepaired
	}
	return m, true
}

// keepBullets takes the model's bullets, discarding any that are evaluative,
// echoed, or not grounded in the specification.
//
// This was all-or-nothing, on the reasoning that dropping two false bullets
// from five leaves a body that reads as complete and is not. Real use settled
// it the other way: on a 58-file change a single mis-attributed symbol threw
// away five accurate bullets and left the user watching the model's work be
// replaced by the mechanical file list. An incomplete description of a change
// is a smaller harm than no description, and the commit message is being
// edited by someone looking at their own diff.
//
// More than half must survive, or it is not a description of the change at
// all. A partial keep reports false, so Source becomes SourceRepaired and the
// footer tells the user something was dropped.
func keepBullets(lines []string, sp spec) (string, bool) {
	var bullets []string
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "- ") || len(strings.TrimSpace(line)) < 4 {
			return "", false
		}
		bullets = append(bullets, line)
	}
	if len(bullets) == 0 {
		return "", false
	}
	if sp.empty() {
		return "", false
	}
	// Drop the bullets that fail rather than the whole body. One bad claim in
	// six used to discard five good ones and leave the user staring at the
	// mechanical file list they had just watched the model replace.
	kept := bullets[:0:0]
	for _, b := range bullets {
		if hasTell(b) || hasEcho(b) || hasEditorial(b) || ground(b, sp) != nil {
			continue
		}
		kept = append(kept, b)
	}
	// Too little survived to call it a description of the change.
	if len(kept) == 0 || len(kept)*2 < len(bullets) {
		return "", false
	}
	return strings.Join(kept, "\n"), len(kept) == len(bullets)
}

// bestOf prefers a partial answer that happens to be a usable message over the
// heuristic, so a generation cancelled just after the header is not thrown
// away.
func bestOf(partial string, fallback Message, prompt string, sp spec, opts Options) Message {
	if m, ok := finish(partial, prompt, sp, opts); ok {
		return withBody(m, fallback)
	}
	return fallback
}

// withBody falls back to the mechanical file summary whenever the model's own
// body did not survive. The message is never left without one.
func withBody(m, fallback Message) Message {
	if m.Body == "" {
		m.Body = fallback.Body
		if m.Source == SourceModel {
			m.Source = SourceRepaired
		}
	}
	return m
}

// firstLine is the model's whole answer, since it was asked for one line —
// after the fence and the chat preamble come off, or "```" would be read as
// the subject.
func firstLine(text string) string {
	lines := unwrap(text)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

// parseMessage splits a rendered message back into its parts, so Source and
// the individual fields survive the round trip through text.
func parseMessage(text string, rules Rules, src Source) Message {
	header, body, _ := strings.Cut(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	m := Message{Subject: strings.TrimSpace(header), Body: strings.Trim(body, "\n"), Source: src}
	if rules.RequireType {
		if match := looseHeaderRe.FindStringSubmatch(m.Subject); match != nil {
			m.Type, m.Scope, m.Subject = match[1], match[2], strings.TrimSpace(match[4])
			if match[3] != "" {
				m.Type += "!"
			}
		}
	}
	return m
}
