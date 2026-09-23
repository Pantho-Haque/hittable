package commitmsg

import (
	"strconv"
	"strings"

	"github.com/hittable/shellapp/internal/llm"
)

// Sampling for a 3B coder model asked to describe a change. Low but not
// greedy: greedy decoding on a small model repeats itself, and anything above
// ~0.3 starts inventing file names.
const (
	draftTemperature = 0.2
	draftMaxTokens   = 400
	promptReserve    = 64 // slack for the role framing the server adds
	minPackBudget    = 1000
	smallChangeFiles = 8 // at or below this, the real diff goes in as well
)

// BuildRequest asks the model for a subject line and a handful of bullets,
// both grounded in a specification rather than in a diff.
//
// The specification is the load-bearing idea. A truncated diff produces "major
// refactoring and new features"; a diff summarised with counts produces a
// transcription of the counts; but each package's own doc comment, written by
// the author in this very change, tells the model what the code is *for* —
// semantic content it could never infer from identifiers, and true by
// construction. Counts are left out entirely, because given a number a 3B
// repeats the number instead of describing behaviour.
//
// The shape matters as much as the content. Every instruction lives in the
// system message and the example is a real alternating turn, so the final user
// turn is a question of a kind the model has just seen answered. An earlier
// version put the instructions at the end of the user turn as trailing prose
// and a 3B continued them instead of answering.
func BuildRequest(d *Digest, opts Options) llm.Request {
	rules := opts.Rules.resolved()
	sp := buildSpec(d)
	lo, hi := bulletRange(sp)

	return llm.Request{
		System: systemPrompt(d, opts, rules, lo, hi),
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: shotSpec},
			{Role: llm.RoleAssistant, Content: shotReply(rules)},
			{Role: llm.RoleUser, Content: userPrompt(d, sp, rules, opts)},
		},
		MaxTokens:   draftMaxTokens,
		Temperature: draftTemperature,
		Stop:        []string{"\n\n\n", "```"},
		Grammar:     grammarFor(rules, opts.Type, lo, hi),
	}
}

// bulletRange scales the body with the change. A one-directory commit gets one
// or two bullets; only a genuinely broad change is asked for four or more.
// Demanding a fixed four would make the common case — a small, focused commit
// — pad itself out, and padding is where invention comes from.
func bulletRange(s spec) (lo, hi int) {
	n := len(s.Blocks)
	if n == 0 {
		return 1, 1
	}
	lo, hi = 1, n+1
	if n >= 4 {
		lo = 3
	}
	if hi > 6 {
		hi = 6
	}
	return lo, hi
}

func systemPrompt(d *Digest, opts Options, rules Rules, lo, hi int) string {
	var b strings.Builder
	b.WriteString("You write git commit messages for a code project.\n\n")
	b.WriteString("You are given the directories that changed, what each package is FOR (its ")
	b.WriteString("\"purpose\" line, written by the author), and the exported functions and types added to it.\n\n")
	b.WriteString("Rules:\n")
	if rules.RequireType {
		b.WriteString("1. First line: type(scope): subject — lower case, imperative, no full stop, at most ")
	} else {
		b.WriteString("1. First line: a short imperative subject — lower case, no full stop, at most ")
	}
	b.WriteString(strconv.Itoa(rules.MaxHeader) + " characters.\n")
	b.WriteString("2. Then a blank line, then " + bulletCount(lo, hi) + ", each starting with \"- \".\n")
	b.WriteString("3. Say what the code now does, leading with the capability rather than the list of names. ")
	b.WriteString("Use the purpose line to say what a package achieves.\n")
	b.WriteString("4. Name only the packages, functions and types you were given, each under the directory it was given for.\n")
	b.WriteString("5. One sentence per bullet. Name at most the two or three identifiers that matter, ")
	b.WriteString("and do not append a list of every name you were given.\n")
	b.WriteString("6. Never mention how many files or lines changed. Invent nothing. No preamble.\n")

	if rules.RequireType && len(rules.Types) > 0 {
		if opts.Type != "" {
			b.WriteString("\nUse the type: " + opts.Type + "\n")
		} else {
			b.WriteString("\nTypes: " + strings.Join(rules.Types, ", ") + "\n")
		}
	}
	if scopes := knownScopes(d); scopes != "" {
		b.WriteString("Scopes in use: " + scopes + "\n")
	}
	// The anchor is the floor: a 3B improves a candidate far more reliably
	// than it invents one. It is a constraint, so it belongs here rather than
	// trailing the specification, where it reads as prose to carry on with.
	if anchor := Generate(d, opts).Subject; anchor != "" {
		b.WriteString("Starting point for the subject, to improve on if the changes support something more specific: " + anchor + "\n")
	}
	if d.Truncated {
		b.WriteString("The summary was shortened to fit. Describe only what it shows.\n")
	}
	return b.String()
}

func bulletCount(lo, hi int) string {
	if lo == hi {
		if lo == 1 {
			return "one bullet"
		}
		return strconv.Itoa(lo) + " bullets"
	}
	return strconv.Itoa(lo) + " to " + strconv.Itoa(hi) + " bullets"
}

// userPrompt is the specification, plus the real diff when the change is small
// enough for it to arrive whole. A truncated diff is worse than none — it is
// what produced "major refactoring and new features" — but for a one or
// two-file commit the hunks are the only place the detail lives.
func userPrompt(d *Digest, sp spec, rules Rules, opts Options) string {
	body := sp.String()
	if len(d.Files) > smallChangeFiles || d.Truncated {
		return body
	}
	budget := DefaultBudget - len(systemPrompt(d, opts, rules, 1, 6)) -
		len(shotSpec) - len(shotReply(rules)) - len(body) - promptReserve
	if budget < minPackBudget {
		return body
	}
	was := d.Truncated
	packed := d.Pack(budget)
	if d.Truncated != was {
		// It did not fit whole after all; keep the specification alone rather
		// than handing over half a diff.
		d.Truncated = was
		return body
	}
	return body + "\n" + packed
}

func knownScopes(d *Digest) string {
	if len(d.Scopes) == 0 {
		return ""
	}
	scopes := d.Scopes
	if len(scopes) > 8 {
		scopes = scopes[:8]
	}
	return strings.Join(scopes, ", ")
}

// The one-shot pair. It is invented rather than taken from history, because it
// has to show the exact shape being asked for — a specification in, a subject
// and bullets out — and the repository's own commits were written from diffs,
// not from specifications. History still supplies the scope vocabulary and the
// anchor above.
const shotSpec = `internal/httpcache (new)
  purpose: Package httpcache stores HTTP responses on disk and serves them until they expire.
  new API: Cache, Get, Put, Evict, TTL
internal/fetch (modified)
  new API: WithCache, RetryPolicy
also touched: cmd/tool`

func shotReply(rules Rules) string {
	body := "\n\n" +
		"- Added internal/httpcache, which keeps HTTP responses on disk until they expire.\n" +
		"- Extended internal/fetch so requests can be served from that cache, via WithCache.\n" +
		"- Wired the new options through cmd/tool."
	if rules.RequireType {
		return "feat(httpcache): cache http responses on disk" + body
	}
	return "cache http responses on disk" + body
}
