package commitmsg

import (
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Source records where a message came from, so the footer can label the draft
// honestly: a user told the model was skipped will trust the one time it was
// not.
type Source int

const (
	SourceHeuristic Source = iota
	SourceModel
	SourceRepaired
)

func (s Source) String() string {
	switch s {
	case SourceModel:
		return "model"
	case SourceRepaired:
		return "model (repaired)"
	default:
		return "heuristic"
	}
}

// Options steers Generate.
type Options struct {
	Type  string // forced type from the UI picker; "" means infer
	Rules Rules
}

// Message is a conventional-commit message in parts. Type is empty in a
// repository that does not follow the convention.
type Message struct {
	Type, Scope, Subject, Body string
	Source                     Source
}

// Header is the first line of the message.
func (m Message) Header() string {
	if m.Type == "" {
		return m.Subject
	}
	if m.Scope == "" {
		return m.Type + ": " + m.Subject
	}
	return m.Type + "(" + m.Scope + "): " + m.Subject
}

// String renders the header, a blank line, and the body — ready to hand to
// `git commit -m`, which splits it into subject and body itself.
func (m Message) String() string {
	if m.Body == "" {
		return m.Header()
	}
	return m.Header() + "\n\n" + m.Body
}

// Generate writes a message from the digest alone: no model, no network. It is
// the default path rather than a fallback, and it always produces something
// Validate accepts.
func Generate(d *Digest, opts Options) Message {
	rules := opts.Rules.resolved()
	m := Message{Source: SourceHeuristic}
	if d == nil || len(d.Files) == 0 {
		m.Subject = "update files"
		if d != nil && typed(d, opts, rules) {
			m.Type = firstNonEmpty(opts.Type, "chore")
		}
		return m
	}

	scope := inferScope(d)
	switch {
	case opts.Type != "":
		m.Type, m.Scope = opts.Type, scope
	case typed(d, opts, rules):
		m.Type, m.Scope = InferType(d), scope
	}
	m.Subject = subjectFor(d, scope)
	m.Body = bodyFor(d)
	// A scope is worth less than the words it would cost.
	if len([]rune(m.Header())) > rules.MaxHeader && m.Scope != "" {
		m.Scope = ""
	}
	m.Subject = capSubject(m, rules.MaxHeader)
	return m
}

// typed reports whether the message should carry a conventional type at all.
// An explicit pick from the UI always wins; otherwise both the rules and the
// repository's own history have to want one, so neither a Rules value with
// RequireType off nor a repository that ignores the convention ever has one
// imposed on it.
func typed(d *Digest, opts Options, rules Rules) bool {
	return opts.Type != "" || (rules.RequireType && d.Convention)
}

// InferType picks the conventional type from the file classes and the branch
// name. It only preselects the picker; it is never silently final.
func InferType(d *Digest) string {
	if d == nil || len(d.Files) == 0 {
		return "chore"
	}
	switch {
	case all(d.Files, func(c Change) bool { return isTest(c.Path) }):
		return "test"
	case all(d.Files, func(c Change) bool { return isDocs(c.Path) }):
		return "docs"
	case all(d.Files, func(c Change) bool { return isStyle(c.Path) }):
		return "style"
	case all(d.Files, func(c Change) bool { return isBuild(c.Path) }):
		return "build"
	case all(d.Files, func(c Change) bool { return isCI(c.Path) }):
		return "ci"
	}
	branch := strings.ToLower(d.Branch)
	for _, p := range []string{"fix/", "bugfix/", "hotfix/"} {
		if strings.HasPrefix(branch, p) {
			return "fix"
		}
	}
	for _, c := range d.Files {
		if c.Status == 'A' && isSource(c.Path) {
			return "feat"
		}
	}
	if all(d.Files, func(c Change) bool { return c.Status == 'D' }) {
		return "refactor"
	}
	return "feat"
}

// ---------- scope ----------

// Segments that name a layer rather than a component, so they lose to anything
// more specific when no learned scope matches.
// maxScope is where a directory name stops being a scope and starts being a
// sentence.
const maxScope = 24

var genericSegments = map[string]bool{
	"src": true, "internal": true, "pkg": true, "lib": true, "app": true,
	"apps": true, "packages": true, "cmd": true, "ui": true, "components": true,
}

// inferScope picks the most meaningful segment of the paths' common directory
// prefix, preferring one the repository's own history already uses.
func inferScope(d *Digest) string {
	segs := commonDirSegments(d.Files)
	if len(segs) == 0 {
		return ""
	}
	best, bestRank := "", len(d.Scopes)
	for _, s := range segs {
		for i, known := range d.Scopes {
			if known == s && i < bestRank {
				best, bestRank = s, i
			}
		}
	}
	if best != "" {
		return best
	}
	for i := len(segs) - 1; i >= 0; i-- {
		if !genericSegments[segs[i]] && len(segs[i]) <= maxScope {
			return segs[i]
		}
	}
	// Every segment only names a layer ("src", "internal"): there is no clear
	// candidate, so the scope is omitted rather than invented.
	return ""
}

func commonDirSegments(files []Change) []string {
	var common []string
	for i, c := range files {
		dir := path.Dir(c.Path)
		var segs []string
		if dir != "." && dir != "/" {
			segs = strings.Split(dir, "/")
		}
		if i == 0 {
			common = segs
			continue
		}
		n := 0
		for n < len(common) && n < len(segs) && common[n] == segs[n] {
			n++
		}
		common = common[:n]
	}
	return common
}

// ---------- subject ----------

func subjectFor(d *Digest, scope string) string {
	files := d.Files
	if all(files, func(c Change) bool { return c.Status == 'R' || c.Status == 'C' }) {
		if len(files) == 1 {
			return "rename " + path.Base(files[0].OldPath) + " to " + path.Base(files[0].Path)
		}
		return "rename " + strconv.Itoa(len(files)) + " files"
	}
	if len(files) == 1 && files[0].Status == 'A' {
		return "add " + path.Base(files[0].Path)
	}
	if all(files, func(c Change) bool { return c.Status == 'D' }) {
		return "remove " + nameList(files, 3)
	}
	if syms := d.newSymbols(); len(syms) > 0 {
		return "add " + joinAnd(syms)
	}
	if all(files, func(c Change) bool { return isTest(c.Path) }) {
		return "add tests for " + orElse(scope, "the codebase")
	}
	return "update " + orElse(scope, "files")
}

// capSubject trims the subject so the whole header fits, cutting at a word
// boundary rather than mid-word.
func capSubject(m Message, max int) string {
	runes := []rune(m.Subject)
	over := len([]rune(m.Header())) - max
	if over <= 0 {
		return m.Subject
	}
	keep := len(runes) - over
	if keep < 1 {
		keep = 1 // the prefix alone is over budget; nothing more we can do here
	}
	s := string(runes[:keep])
	if i := strings.LastIndexByte(s, ' '); i > 0 {
		s = s[:i]
	}
	return strings.TrimRight(strings.TrimSpace(s), ",.-")
}

// ---------- body ----------

// bodyFor lists one bullet per directory: the basenames that changed and the
// line counts. It states what changed, never why — the thinking stays human.
func bodyFor(d *Digest) string {
	var order []string
	names := map[string][]string{}
	added, removed := map[string]int{}, map[string]int{}
	for _, c := range d.Files {
		dir := path.Dir(c.Path)
		if dir == "." || dir == "/" {
			dir = ""
		}
		if _, ok := names[dir]; !ok {
			order = append(order, dir)
		}
		names[dir] = append(names[dir], path.Base(c.Path))
		added[dir] += c.Added
		removed[dir] += c.Removed
	}
	var lines []string
	for _, dir := range order {
		ns := names[dir]
		shown := ns
		if len(shown) > 4 {
			shown = append(append([]string{}, shown[:4]...), "…")
		}
		line := "- "
		if dir != "" {
			line += dir + ": "
		}
		line += strings.Join(shown, ", ")
		line += " (+" + strconv.Itoa(added[dir]) + " −" + strconv.Itoa(removed[dir]) + ")"
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// ---------- file classes ----------

func isTest(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".spec.") || strings.Contains(base, ".test.") ||
		strings.Contains(p, "__tests__/") || strings.HasPrefix(p, "test/") ||
		strings.Contains(p, "/test/")
}

func isDocs(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, ".md") || strings.HasSuffix(base, ".mdx") ||
		strings.HasPrefix(p, "docs/") || strings.Contains(p, "/docs/") ||
		base == "LICENSE"
}

func isStyle(p string) bool {
	for _, ext := range []string{".css", ".scss", ".sass", ".less"} {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

func isBuild(p string) bool {
	base := path.Base(p)
	switch base {
	case "go.mod", "go.sum", "package.json", "Dockerfile", "Makefile", "docker-compose.yml":
		return true
	}
	for _, n := range demotedNames {
		if base == n {
			return true
		}
	}
	return false
}

func isCI(p string) bool {
	return strings.Contains(p, ".github/workflows/") || strings.HasPrefix(p, ".circleci/") ||
		path.Base(p) == ".gitlab-ci.yml"
}

var sourceExts = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".py": true,
	".rs": true, ".java": true, ".rb": true, ".c": true, ".h": true, ".cc": true,
	".cpp": true, ".cs": true, ".swift": true, ".kt": true, ".php": true,
	".sh": true, ".sql": true, ".vue": true, ".svelte": true,
}

// isCodeFile is extension-only: a _test.go file is still code, it just has
// symbols that do not count.
func isCodeFile(p string) bool {
	return sourceExts[strings.ToLower(path.Ext(p))]
}

func isSource(p string) bool {
	return isCodeFile(p) && !isTest(p)
}

// ---------- exported symbols ----------

var symbolRes = []*regexp.Regexp{
	regexp.MustCompile(`^\+func (?:\([^)]+\) )?([A-Z]\w+)`),
	regexp.MustCompile(`^\+type ([A-Z]\w+)`),
	regexp.MustCompile(`^\+export (?:async )?(?:function|const|class|type|interface) (\w+)`),
	regexp.MustCompile(`^\+(?:def|class) (\w+)`),
	regexp.MustCompile(`^\+pub (?:fn|struct|enum) (\w+)`),
}

// testFuncRe matches the functions a _test.go file exports because the
// toolchain demands it, not because they are the point of the change.
var testFuncRe = regexp.MustCompile(`^(Test|Benchmark|Fuzz|Example)`)

// exportedSymbols returns the newly declared exported symbols of a file, in
// declaration order. Collect fills Symbols from the untrimmed diff; a digest
// built by hand falls back to whatever the hunks still show.
func exportedSymbols(c Change) []string {
	if len(c.Symbols) > 0 {
		return c.Symbols
	}
	if len(c.Hunks) == 0 {
		return nil
	}
	return scanSymbols(strings.Split(strings.Join(c.Hunks, "\n"), "\n"), c.Path)
}

// scanSymbols reads added lines for new exported declarations. In a test file
// the Test/Benchmark/Fuzz/Example functions are skipped, so they never earn
// the file a packing promotion either.
func scanSymbols(lines []string, path string) []string {
	// Only code declares symbols. A fenced Go block inside a design document
	// matched "+func Runner" and put it in the spec as an API of docs/, which
	// is a claim about a directory that contains no code at all.
	if !isCodeFile(path) {
		return nil
	}
	test := isTest(path)
	var out []string
	seen := map[string]bool{}
	for _, line := range lines {
		for _, re := range symbolRes {
			m := re.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if name := m[1]; !seen[name] && !(test && testFuncRe.MatchString(name)) {
				seen[name] = true
				out = append(out, name)
			}
			break
		}
	}
	return out
}

// newSymbols collects the symbols worth naming a commit after, phrased the way
// the repository's own subjects are: camelCase split and lowercased, so
// FoldAll reads "fold all" and passes commitlint's subject-case rule.
//
// Two rules keep the choice from being arbitrary. Test files are skipped
// entirely — a change is never named after its tests, and a tests-only change
// falls through to the "add tests for x" rule below. The rest are visited
// best-ranked file first and in declaration order within a file, so the symbol
// that names the commit is the first one declared in the file that changed
// most, not whatever the scan happened to end on.
func (d *Digest) newSymbols() []string {
	var out []string
	seen := map[string]bool{}
	for _, i := range d.ranked() {
		c := d.Files[i]
		if isTest(c.Path) {
			continue
		}
		for _, s := range exportedSymbols(c) {
			w := splitCamel(s)
			if !seen[w] {
				seen[w] = true
				out = append(out, w)
			}
		}
	}
	return out
}

func splitCamel(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		upper := unicode.IsUpper(r)
		prevLower := i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]))
		nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
		prevUpper := i > 0 && unicode.IsUpper(runes[i-1])
		if i > 0 && upper && (prevLower || (prevUpper && nextLower)) {
			b.WriteByte(' ')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// ---------- small helpers ----------

func all(files []Change, ok func(Change) bool) bool {
	if len(files) == 0 {
		return false
	}
	for _, c := range files {
		if !ok(c) {
			return false
		}
	}
	return true
}

func nameList(files []Change, max int) string {
	var names []string
	for _, c := range files {
		names = append(names, path.Base(c.Path))
	}
	if len(names) > max {
		return strings.Join(names[:max], ", ") + " and " + strconv.Itoa(len(names)-max) + " more"
	}
	return joinAnd(names)
}

func joinAnd(items []string) string {
	if len(items) > 3 {
		items = items[:3]
	}
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
	}
}

func orElse(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
