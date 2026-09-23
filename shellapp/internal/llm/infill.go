package llm

// infillBody builds llama.cpp's /infill request: fill-in-the-middle, where the
// model is given the text on both sides of the cursor rather than being asked
// to continue the text before it. This is what makes inline completion in an
// editor produce something that fits what follows.
//
// The route is not OpenAI-compatible: the fields are input_prefix /
// input_suffix, the token budget is n_predict, and the reply carries its text
// at the top level as "content" rather than under "choices".
func infillBody(req Request, stream bool) map[string]any {
	b := map[string]any{
		"input_prefix": req.Prompt,
		"input_suffix": req.Suffix,
		"stream":       stream,
	}
	// llama.cpp takes an optional top-level prompt as extra context placed
	// ahead of the prefix — the only place a System string can go on a route
	// that has no roles.
	if req.System != "" {
		b["prompt"] = req.System
	}
	if req.MaxTokens > 0 {
		b["n_predict"] = req.MaxTokens
	}
	applySampling(b, req)
	return b
}
