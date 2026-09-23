package llm

import (
	"encoding/json"
	"strings"
)

// buildBody produces the JSON body for one route. Fields left at their zero
// value are omitted, so the server's own default applies — except temperature,
// which is always sent (see Request).
func buildBody(rt route, req Request, stream bool) map[string]any {
	switch rt {
	case routeInfill:
		return infillBody(req, stream)
	case routeCompletions:
		return completionBody(req, stream)
	default:
		return chatBody(req, stream)
	}
}

// chatBody builds /v1/chat/completions. System becomes the leading system
// message.
func chatBody(req Request, stream bool) map[string]any {
	msgs := make([]Message, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, Message{Role: RoleSystem, Content: req.System})
	}
	msgs = append(msgs, req.Messages...)

	b := map[string]any{
		"messages": msgs,
		"stream":   stream,
	}
	if req.MaxTokens > 0 {
		b["max_tokens"] = req.MaxTokens
	}
	applySampling(b, req)
	applyStreamOptions(b, stream)
	return b
}

// completionBody builds /v1/completions. There is no system role on this route,
// so System is prepended to the prompt rather than dropped on the floor.
func completionBody(req Request, stream bool) map[string]any {
	prompt := req.Prompt
	if req.System != "" {
		prompt = req.System + "\n\n" + prompt
	}

	b := map[string]any{
		"prompt": prompt,
		"stream": stream,
	}
	if req.MaxTokens > 0 {
		b["max_tokens"] = req.MaxTokens
	}
	applySampling(b, req)
	applyStreamOptions(b, stream)
	return b
}

// applySampling adds the knobs every route shares.
func applySampling(b map[string]any, req Request) {
	if req.Model != "" {
		b["model"] = req.Model
	}
	b["temperature"] = req.Temperature
	if req.TopP > 0 {
		b["top_p"] = req.TopP
	}
	if len(req.Stop) > 0 {
		b["stop"] = req.Stop
	}
	if req.Seed != 0 {
		b["seed"] = req.Seed
	}
	if req.Grammar != "" {
		b["grammar"] = req.Grammar
	}
	applyFormat(b, req.Format)
}

// applyStreamOptions asks for the usage block on the final frame; without it an
// OpenAI-compatible stream reports no token counts at all.
func applyStreamOptions(b map[string]any, stream bool) {
	if stream {
		b["stream_options"] = map[string]any{"include_usage": true}
	}
}

// applyFormat maps Request.Format onto response_format: "json" is the loose
// JSON-object mode, and anything that looks like a JSON object is taken as a
// schema. Anything else is ignored rather than sent as a malformed field.
func applyFormat(b map[string]any, format string) {
	f := strings.TrimSpace(format)
	switch {
	case f == "":
	case strings.EqualFold(f, "json"):
		b["response_format"] = map[string]any{"type": "json_object"}
	case strings.HasPrefix(f, "{"):
		b["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "response",
				"strict": true,
				"schema": json.RawMessage(f),
			},
		}
	}
}
