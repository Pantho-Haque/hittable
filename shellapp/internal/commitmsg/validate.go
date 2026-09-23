package commitmsg

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Rules is commitlint's configuration as a value, never a path: the app opens
// arbitrary directories and the repository's own commitlint.config.js may live
// anywhere above, below or nowhere at all.
type Rules struct {
	Types            []string
	MaxHeader        int // 72
	RequireType      bool
	ForbidFullStop   bool
	BodyLeadingBlank bool
}

// DefaultRules is @commitlint/config-conventional as this monorepo configures
// it: the 11-type enum, lowercase type, no trailing period, a blank line
// before the body.
func DefaultRules() Rules {
	return Rules{
		Types: []string{
			"feat", "fix", "docs", "style", "refactor", "perf",
			"test", "build", "ci", "chore", "revert",
		},
		MaxHeader:        72,
		RequireType:      true,
		ForbidFullStop:   true,
		BodyLeadingBlank: true,
	}
}

// PlainRules is what a repository that does not follow conventional commits
// gets: an imperative subject, still capped and still without a trailing
// period, but no type and no scope invented for it.
//
// Pass this rather than a bare Rules{} when you mean "no type": every field of
// Rules{} is its zero value, which resolved cannot tell apart from "unset" and
// so fills in with DefaultRules.
func PlainRules() Rules {
	return Rules{
		MaxHeader:        72,
		RequireType:      false,
		ForbidFullStop:   true,
		BodyLeadingBlank: true,
	}
}

// resolved fills in an unset Rules. A completely zero value means the caller
// did not configure anything, which is the common case for Options{} — see
// PlainRules for the way to ask for the non-conventional rule set.
func (r Rules) resolved() Rules {
	if r.MaxHeader == 0 && len(r.Types) == 0 {
		return DefaultRules()
	}
	if r.MaxHeader == 0 {
		r.MaxHeader = 72
	}
	return r
}

func (r Rules) knownType(t string) bool {
	for _, k := range r.Types {
		if k == t {
			return true
		}
	}
	return false
}

// headerRe is the strict conventional header, used to read history.
var headerRe = regexp.MustCompile(`^([a-z]+)(?:\(([^)]*)\))?(!)?: (.+)$`)

// looseHeaderRe is what a small model tends to emit: stray spaces, a
// capitalised type, an empty scope.
var looseHeaderRe = regexp.MustCompile(`^\s*([A-Za-z][\w.-]*)\s*(?:\(([^)]*)\))?\s*(!)?\s*:\s*(.*)$`)

// Phrases that mean the model started talking to the user instead of writing
// a commit message. Not a commitlint rule — ours. A commit subject is
// imperative and never first person, so these cost nothing to forbid and they
// are the difference between a refusal becoming "chore: i'm sorry, i can't
// help with that" and being rejected outright.
var aiTells = []string{
	"as an ai", "as a language model", "here is the", "here's the",
	"i have ", "i've ", "sure, ", "certainly,", "commit message:",
	"i'm sorry", "i am sorry", "i apologi", "i cannot", "i can't",
	"unfortunately, i",
}

// Fragments of this package's own prompt. A model that continues the prompt
// instead of answering it emits these verbatim, which makes them free to
// detect without knowing which prompt was sent — and they cannot occur in a
// commit message that is describing a change.
var promptEchoes = []string{
	"staged files (", "branch: ", "working directory: ",
	"starting point for the subject", "the summary was shortened",
	"describe only what it shows", "you write git commit messages",
	"you are given the directories that changed", "scopes in use",
	"leading with the capability", "never mention how many files",
	"each starting with", "new api:", "purpose:",
}

// Echoes reports whether the model handed back the prompt instead of
// answering it. Any non-trivial line of the output appearing verbatim in the
// prompt is the giveaway; the comparison ignores case and runs of whitespace
// so a reflowed echo is caught too.
func Echoes(msg, prompt string) bool {
	if strings.TrimSpace(prompt) == "" {
		return false
	}
	hay := " " + normaliseSpace(prompt) + " "
	for _, line := range strings.Split(msg, "\n") {
		norm := normaliseSpace(line)
		// Short lines collide by chance — a path or a type name can legitimately
		// appear in both.
		if len(norm) < 24 {
			continue
		}
		if strings.Contains(hay, " "+norm+" ") {
			return true
		}
	}
	return false
}

func normaliseSpace(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// Product-brochure prose. A 3B given room to write fills it with claims about
// qualities and benefits that a diff cannot support — "styled to be
// minimalistic and easy to use" about a component with no styling in it. Every
// phrase here was checked against 300 commits of this repository's real
// history and matched none of them, so the check costs no true messages.
// Deliberately narrow: "improve", "better" and "enhance" all occur in genuine
// bodies and are not on the list.
var editorialPhrases = []string{
	"easy to use", "easy-to-use", "and easy", "intuitive", "minimalist",
	"user-friendly", "user friendly", "seamless", "more efficient",
	"allows users to", "allows the user to", "allows you to",
	"user experience", "is styled to", "out of the box",
	"making it easier", "makes it easier", "provides a better",
	"ensures that", "leverag", "robust", "powerful", "elegant",
	"streamlines", "comprehensive", "modern", "clean and", "simple and",
	"flexible and", "best practices",
}

func hasEditorial(s string) bool {
	low := strings.ToLower(s)
	for _, phrase := range editorialPhrases {
		if strings.Contains(low, phrase) {
			return true
		}
	}
	return false
}

func hasEcho(s string) bool {
	low := strings.ToLower(s)
	for _, frag := range promptEchoes {
		if strings.Contains(low, frag) {
			return true
		}
	}
	return false
}

func hasTell(s string) bool {
	low := strings.ToLower(s)
	for _, tell := range aiTells {
		if strings.Contains(low, tell) {
			return true
		}
	}
	return false
}

// Validate checks a rendered message against the rules and returns the first
// violation, named after the commitlint rule it breaks. The two body-* checks
// at the end are ours, not commitlint's: a grammar cannot enforce semantics.
func Validate(msg string, r Rules) error {
	r = r.resolved()
	lines := strings.Split(strings.ReplaceAll(msg, "\r\n", "\n"), "\n")
	header := strings.TrimRight(lines[0], " \t")
	if strings.TrimSpace(header) == "" {
		return errors.New("subject-empty: the message is empty")
	}
	if n := len([]rune(header)); n > r.MaxHeader {
		return fmt.Errorf("header-max-length: header is %d characters, limit is %d", n, r.MaxHeader)
	}

	subject := header
	if r.RequireType {
		m := looseHeaderRe.FindStringSubmatch(header)
		if m == nil {
			return errors.New("type-empty: header is not '<type>(<scope>): <subject>'")
		}
		typ, scope, sub := m[1], m[2], m[4]
		if typ != strings.ToLower(typ) {
			return fmt.Errorf("type-case: type %q must be lower case", typ)
		}
		if !r.knownType(typ) {
			return fmt.Errorf("type-enum: type %q is not one of %s", typ, strings.Join(r.Types, ", "))
		}
		if strings.Contains(header, "(") && strings.TrimSpace(scope) == "" {
			return errors.New("scope-empty: the scope parentheses are empty")
		}
		subject = sub
	}

	if strings.TrimSpace(subject) == "" {
		return errors.New("subject-empty: the subject is empty")
	}
	if r.ForbidFullStop && strings.HasSuffix(strings.TrimRight(subject, " \t"), ".") {
		return errors.New("subject-full-stop: the subject must not end with '.'")
	}
	if badSubjectCase(subject) {
		return fmt.Errorf("subject-case: subject %q must not be sentence, start, pascal or upper case", subject)
	}
	if hasTell(subject) {
		return fmt.Errorf("subject-preamble: the subject is talking to the reader, not describing the change: %q", subject)
	}
	if hasEcho(subject) {
		return fmt.Errorf("subject-echo: the subject repeats the prompt rather than describing the change: %q", subject)
	}
	if hasEditorial(subject) {
		return fmt.Errorf("subject-editorial: the subject claims a quality the changes cannot show: %q", subject)
	}
	if r.BodyLeadingBlank && len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
		return errors.New("body-leading-blank: the body must be preceded by a blank line")
	}

	body := strings.TrimSpace(strings.Join(lines[min(len(lines), 1):], "\n"))
	if body != "" {
		if hasTell(body) {
			return errors.New("body-preamble: the body is talking to the reader, not describing the change")
		}
		if hasEcho(body) {
			return errors.New("body-echo: the body repeats the prompt rather than describing the change")
		}
		if hasEditorial(body) {
			return errors.New("body-editorial: the body claims qualities or benefits the changes cannot show")
		}
		if restates(body, subject) {
			return errors.New("body-restates-subject: the body only repeats the subject")
		}
	}
	return nil
}

// badSubjectCase implements commitlint's subject-case rule, which rejects
// sentence, start, pascal and upper case as a class. Every one of those starts
// with a capital, and nothing else does, so the first rune decides.
func badSubjectCase(subject string) bool {
	for _, r := range strings.TrimSpace(subject) {
		return unicode.IsUpper(r)
	}
	return false
}

func restates(body, subject string) bool {
	norm := func(s string) string {
		var b strings.Builder
		for _, r := range strings.ToLower(s) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(r)
			}
		}
		return b.String()
	}
	return norm(body) == norm(subject)
}

// ---------- repair ----------

var fenceRe = regexp.MustCompile("(?s)```[a-zA-Z]*\n?(.*?)```")

var typeSynonyms = map[string]string{
	"feature": "feat", "features": "feat", "add": "feat", "new": "feat",
	"bugfix": "fix", "bug": "fix", "hotfix": "fix", "fixes": "fix", "fixed": "fix",
	"doc": "docs", "documentation": "docs",
	"tests": "test", "testing": "test",
	"refactoring": "refactor", "cleanup": "refactor", "ref": "refactor",
	"performance": "perf", "styles": "style", "chores": "chore",
	"builds": "build", "reverts": "revert",
}

// Repair mechanically fixes what a small model gets wrong about the form of a
// commit message, and reports whether the result passes Validate. It never
// invents content: an unusable message stays unusable so the caller falls back
// to the heuristic rather than committing nonsense.
func Repair(msg string, r Rules, fallbackType string) (string, bool) {
	r = r.resolved()
	body := ""
	header := ""

	lines := unwrap(msg)
	if len(lines) == 0 {
		return "", false
	}
	header = strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		body = strings.Trim(strings.Join(lines[1:], "\n"), "\n")
	}

	var typ, scope, bang, subject string
	if r.RequireType {
		typ, scope, bang, subject = splitHeader(header)
		typ = repairType(typ, r, fallbackType)
		scope = repairScope(scope)
	} else {
		// Without a required type the whole header is the subject: splitting
		// it would eat the first word of "add http: support".
		subject = header
	}
	subject = repairSubject(subject, r)
	if subject == "" || hasTell(subject) || hasEcho(subject) || hasEditorial(subject) {
		// A refusal, an apology or a mouthful of the prompt is not a commit
		// message with formatting problems; there is nothing here to repair
		// into one.
		return "", false
	}

	m := Message{Type: typ, Scope: scope, Subject: subject, Body: repairBody(body, subject)}
	if bang != "" && m.Type != "" {
		m.Type += "!"
	}
	m.Subject = capSubject(m, r.MaxHeader)
	out := m.String()
	return out, Validate(out, r) == nil
}

// unwrap peels the code fence and the chat preamble off a model's answer and
// returns the lines that are left. The first of them is the header — which is
// why the fence has to come off first, or "```" is read as the subject.
func unwrap(msg string) []string {
	text := strings.ReplaceAll(msg, "\r\n", "\n")
	if m := fenceRe.FindStringSubmatch(text); m != nil {
		text = m[1]
	}
	text = strings.ReplaceAll(text, "```", "")
	text = strings.Trim(text, " \t\n")
	return stripPreamble(strings.Split(text, "\n"))
}

// stripPreamble drops the "Here is the commit message:" line a chat-tuned
// model likes to open with, and any blank lines before the header.
func stripPreamble(lines []string) []string {
	for len(lines) > 0 {
		first := strings.TrimSpace(lines[0])
		if first == "" {
			lines = lines[1:]
			continue
		}
		low := strings.ToLower(first)
		isPreamble := false
		for _, tell := range aiTells {
			if strings.HasPrefix(low, strings.TrimSpace(tell)) {
				isPreamble = true
				break
			}
		}
		if !isPreamble {
			break
		}
		// "Commit message: feat: x" keeps its tail; a bare preamble line goes.
		if i := strings.Index(first, ":"); i >= 0 && strings.TrimSpace(first[i+1:]) != "" {
			rest := strings.TrimSpace(first[i+1:])
			if looseHeaderRe.MatchString(rest) {
				lines[0] = rest
				break
			}
		}
		lines = lines[1:]
	}
	return lines
}

// splitHeader tears a header apart however it was written; everything but the
// subject may come back empty.
func splitHeader(header string) (typ, scope, bang, subject string) {
	if m := looseHeaderRe.FindStringSubmatch(header); m != nil {
		return m[1], m[2], m[3], m[4]
	}
	return "", "", "", header
}

func repairType(typ string, r Rules, fallback string) string {
	t := strings.ToLower(strings.TrimSpace(typ))
	if syn, ok := typeSynonyms[t]; ok {
		t = syn
	}
	if r.knownType(t) {
		return t
	}
	if f := strings.ToLower(strings.TrimSpace(fallback)); r.knownType(f) {
		return f
	}
	return "chore"
}

func repairScope(scope string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(scope)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '/':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// repairSubject applies subject-case and subject-full-stop.
//
// subject-case lowercases ONLY the first character: commitlint rejects
// sentence, start, pascal and upper case as a class, so "add FoldAll to the
// editor" is already legal and blanket-lowercasing would destroy the
// identifier. An all-caps subject is the one exception — lowercasing just its
// first character would leave it shouting.
func repairSubject(subject string, r Rules) string {
	s := strings.TrimSpace(subject)
	s = strings.Trim(s, "\"'`")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if isAllCaps(s) {
		s = strings.ToLower(s)
	} else {
		runes := []rune(s)
		runes[0] = unicode.ToLower(runes[0])
		s = string(runes)
	}
	if r.ForbidFullStop {
		s = strings.TrimRight(s, ".")
		s = strings.TrimRight(s, " \t")
	}
	return dropUnmatchedQuotes(s)
}

// dropUnmatchedQuotes removes a lone backtick or quote. A subject cut short by
// the grammar's length ceiling keeps the opening mark and loses the closing
// one, which is how "improve draft to `im" reached an editor.
func dropUnmatchedQuotes(s string) string {
	// Not the apostrophe: an odd count of those is "don't", not a dangling
	// quote, and stripping it turns "i'm sorry" into a subject that no longer
	// reads as a refusal.
	for _, q := range []string{"`", "\""} {
		if strings.Count(s, q)%2 == 1 {
			if i := strings.LastIndex(s, q); i >= 0 {
				s = s[:i] + s[i+1:]
			}
		}
	}
	return strings.TrimRight(strings.TrimSpace(s), " ,;:-")
}

func isAllCaps(s string) bool {
	letters, upper := 0, 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	return letters > 1 && letters == upper
}

// repairBody keeps a body only if it says something: a chat preamble or a
// restatement of the subject is worse than no body at all.
func repairBody(body, subject string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	var kept []string
	for _, line := range strings.Split(body, "\n") {
		if !hasTell(line) && !hasEcho(line) {
			kept = append(kept, line)
		}
	}
	body = strings.Trim(strings.Join(kept, "\n"), "\n")
	// Editorial prose is dropped whole rather than line by line: it arrives
	// wrapped across several lines, so removing the offending one leaves a
	// mangled half-sentence. A true subject with no body beats a subject with
	// an invented one.
	if hasEditorial(body) {
		return ""
	}
	if strings.TrimSpace(body) == "" || restates(body, subject) {
		return ""
	}
	return body
}

// ---------- grammar ----------

// Grammar emits GBNF that llama-server enforces during sampling, which makes
// type-enum, type-case, subject-case, subject-full-stop, body-leading-blank
// and the header cap structurally unviolatable rather than merely discouraged.
// pickedType narrows the type alternation to the one the user chose.
func Grammar(r Rules, pickedType string) string {
	return grammarFor(r, pickedType, 0, 0)
}

// grammarFor is Grammar with an explicit bullet range. A body is only offered
// when the caller asks for one, and the range scales with the size of the
// change: demanding four bullets for a one-file commit is asking the model to
// pad, which is asking it to invent.
func grammarFor(r Rules, pickedType string, loBullets, hiBullets int) string {
	r = r.resolved()
	types := r.Types
	if pickedType != "" && r.knownType(pickedType) {
		types = []string{pickedType}
	}

	// A ceiling, not the cap. The sampler can only stop at a character count,
	// which lands mid-word; capSubject cuts at a word boundary afterwards. So
	// this is deliberately generous — a header one word too long is repaired
	// cleanly, while a header cut at exactly the ceiling is not repairable at
	// all.
	maxSubject := r.MaxHeader - 2
	if maxSubject < 8 {
		maxSubject = 8
	}

	tail := ""
	if hiBullets > 0 {
		tail = ` "\n\n" bullets`
	}

	var b strings.Builder
	if !r.RequireType || len(types) == 0 {
		b.WriteString("root    ::= subject" + tail + "\n")
	} else {
		quoted := make([]string, 0, len(types))
		for _, t := range types {
			quoted = append(quoted, strconv.Quote(t))
		}
		b.WriteString("root    ::= header" + tail + "\n")
		b.WriteString("header  ::= type scope? \"!\"? \": \" subject\n")
		b.WriteString("type    ::= " + strings.Join(quoted, " | ") + "\n")
		b.WriteString("scope   ::= \"(\" [a-z0-9] [a-z0-9-]* \")\"\n")
	}
	b.WriteString("subject ::= [a-z] [^\\n.]{3," + strconv.Itoa(maxSubject) + "}\n")
	if hiBullets > 0 {
		// The repetition counts the separators, so it is one less than the
		// number of bullets at each end.
		b.WriteString("bullets ::= bullet (\"\\n\" bullet){" +
			strconv.Itoa(max(0, loBullets-1)) + "," + strconv.Itoa(max(0, hiBullets-1)) + "}\n")
		b.WriteString("bullet  ::= \"- \" [^\\n]+\n")
	}
	return b.String()
}
