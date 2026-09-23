package commitmsg

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Bounds on the specification. The API cap is the important one: a model given
// twelve names appends all twelve to its prose, and the bullet stops being a
// description and becomes a list.
const (
	specAPIMax    = 6
	specMinorSize = 3 // lines changed below which a directory is "also touched"
	specMaxBlocks = 12
)

// spec is what the model is actually asked to describe: one block per
// directory, what the package is for, and what it now exposes.
//
// Deliberately no file counts and no line counts anywhere. Given numbers a 3B
// transcribes them — "added internal/commitmsg with 15 files, including 4385
// new lines" — instead of saying what the code does. Given a purpose and some
// names, it describes behaviour.
type spec struct {
	Blocks []specBlock
	Minor  []string // directories merged into the closing bullet
}

type specBlock struct {
	Dir     string
	New     bool
	Purpose string   // the package doc comment, written by the author
	API     []string // exported symbols this change adds
}

// buildSpec groups the staged changes by directory, largest first.
func buildSpec(d *Digest) spec {
	type group struct {
		dir            string
		files, newFile int
		added, removed int
		code           bool
		api            []string
		seen           map[string]bool
		doc            string
	}
	groups := map[string]*group{}
	var order []string

	for _, c := range d.Files {
		if IsSecret(c.Path) {
			continue
		}
		dir := path.Dir(c.Path)
		if dir == "." {
			dir = path.Base(c.Path)
		}
		g, ok := groups[dir]
		if !ok {
			g = &group{dir: dir, seen: map[string]bool{}}
			groups[dir], order = g, append(order, dir)
		}
		g.files++
		g.code = g.code || isCodeFile(c.Path)
		if c.Status == 'A' {
			g.newFile++
		}
		g.added, g.removed = g.added+c.Added, g.removed+c.Removed
		if g.doc == "" {
			g.doc = c.Doc
		}
		for _, sym := range c.Symbols {
			if !g.seen[sym] {
				g.seen[sym] = true
				g.api = append(g.api, sym)
			}
		}
	}

	// Largest first, ties broken by the order git listed them, so the same
	// staged change always produces the same specification.
	rank := map[string]int{}
	for i, dir := range order {
		rank[dir] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := groups[order[i]], groups[order[j]]
		if a.added+a.removed != b.added+b.removed {
			return a.added+a.removed > b.added+b.removed
		}
		return rank[a.dir] < rank[b.dir]
	})

	var s spec
	for _, dir := range order {
		g := groups[dir]
		// A directory with nothing to show does not deserve a bullet of its
		// own; it becomes part of the closing one, where it reads as the
		// incidental change it is. A directory of prose has nothing to show
		// however many lines it gained — a block reading only "docs (new)"
		// produced the bullet "Added docs, internal/hithome, and
		// internal/hithome", which is padding, and padding is where invention
		// starts.
		trivial := g.added+g.removed <= specMinorSize || !g.code
		if trivial && len(g.api) == 0 && g.doc == "" {
			s.Minor = append(s.Minor, dir)
			continue
		}
		if len(s.Blocks) >= specMaxBlocks {
			s.Minor = append(s.Minor, dir)
			continue
		}
		api := g.api
		if len(api) > specAPIMax {
			api = api[:specAPIMax]
		}
		s.Blocks = append(s.Blocks, specBlock{
			Dir:     dir,
			New:     g.newFile == g.files,
			Purpose: g.doc,
			API:     api,
		})
	}
	return s
}

// String renders the specification for the prompt.
func (s spec) String() string {
	var b strings.Builder
	for _, blk := range s.Blocks {
		what := "modified"
		if blk.New {
			what = "new"
		}
		fmt.Fprintf(&b, "%s (%s)\n", blk.Dir, what)
		if blk.Purpose != "" {
			fmt.Fprintf(&b, "  purpose: %s\n", blk.Purpose)
		}
		if len(blk.API) > 0 {
			fmt.Fprintf(&b, "  new API: %s\n", strings.Join(blk.API, ", "))
		}
	}
	if len(s.Minor) > 0 {
		minor := s.Minor
		if len(minor) > 8 {
			minor = minor[:8]
		}
		fmt.Fprintf(&b, "also touched: %s\n", strings.Join(minor, ", "))
	}
	return b.String()
}

func (s spec) empty() bool { return len(s.Blocks) == 0 && len(s.Minor) == 0 }

// quotable is every token of the specification as the model was shown it,
// purpose lines included. Anything here is something we handed over, so the
// model repeating it is faithfulness rather than invention.
func (s spec) quotable() map[string]bool {
	set := map[string]bool{}
	for _, tok := range specTokens(s.String()) {
		set[strings.ToLower(tok)] = true
	}
	return set
}

// dirs is every path the specification mentions, including parents, so a
// bullet may legitimately say "internal" when the block is "internal/llm".
func (s spec) dirs() map[string]bool {
	set := map[string]bool{}
	add := func(dir string) {
		for d := dir; d != "." && d != "/" && d != ""; d = path.Dir(d) {
			set[d] = true
		}
	}
	for _, blk := range s.Blocks {
		add(blk.Dir)
	}
	for _, dir := range s.Minor {
		add(dir)
	}
	return set
}

// vocabulary is every word the specification contains, lower-cased. An
// identifier-shaped token has to come from here: the model was given these
// names and no others, so anything else it writes in that shape it made up.
//
// The comparison ignores case because "the GitPanel component" is a rendering
// of the gitpanel package, not a fabricated symbol, while "CloseAI" matches
// nothing however it is folded. Attribution is still checked exactly — that
// question is about identity, not spelling.
func (s spec) vocabulary() map[string]bool {
	set := map[string]bool{}
	for _, tok := range specTokens(strings.ToLower(strings.ReplaceAll(s.String(), "\n", " "))) {
		set[tok] = true
		// The vocabulary is an allow-list, so it is generous: a purpose line
		// saying "OpenAI-compatible" licenses a bullet saying "OpenAI".
		for _, seg := range strings.FieldsFunc(tok, func(r rune) bool {
			return r == '/' || r == '-' || r == '.' || r == '_'
		}) {
			set[seg] = true
		}
	}
	return set
}

// identifierShaped reports whether a token is code rather than prose: an
// internal capital with lower case elsewhere. "Added" and "Modified" are
// prose; "GenChunkMsg" and "CloseAI" are not. All-capital tokens are acronyms
// — HTTP, JSON, API — and are left alone, including in the plural, because
// "LLMs" is a word and rejecting a true bullet costs more than letting one
// acronym through.
func identifierShaped(tok string) bool {
	if strings.HasSuffix(tok, "s") {
		tok = tok[:len(tok)-1]
	}
	if len(tok) < 3 || !isLetter(rune(tok[0])) {
		return false
	}
	var lower, innerUpper bool
	for i, r := range tok {
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z' && i > 0:
			innerUpper = true
		}
	}
	return lower && innerUpper
}

func isLetter(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }

// owners maps each symbol to the directories that actually declare it.
func (s spec) owners() map[string]map[string]bool {
	owners := map[string]map[string]bool{}
	for _, blk := range s.Blocks {
		for _, sym := range blk.API {
			if owners[sym] == nil {
				owners[sym] = map[string]bool{}
			}
			owners[sym][blk.Dir] = true
		}
	}
	return owners
}

// ground checks a model-written body against the specification it was given.
//
// This is the check that replaces a phrase filter with something that can
// actually be wrong or right. Two claims are verifiable from the spec alone: a
// path the change never touched, and a symbol attributed to the wrong
// directory — "modified cmd/hittable to include new APIs for CloseAI", where
// CloseAI is real but lives in ui/screens. Both read as authoritative and both
// are false, and no list of words could have separated them from the truth.
//
// Three things are checked: a path must be one the change touched, an
// identifier-shaped token must appear somewhere in the specification, and a
// symbol named beside a directory must be a symbol of that directory. Ordinary
// English is never examined, because prose does not put a capital in the
// middle of a word.
func ground(body string, s spec) error {
	dirs, owners, vocab := s.dirs(), s.owners(), s.vocabulary()
	quoted := s.quotable()

	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		tokens := specTokens(line)

		// Which blocks is this bullet talking about, and did it invent any of
		// the names it used?
		refs := map[string]bool{}
		for _, tok := range tokens {
			// A token we put in front of the model is grounded by definition.
			// Purpose lines are the author's own prose and routinely contain
			// paths that are not part of the change — ~/.hittable/config.json,
			// say. Rejecting the model for quoting our own specification threw
			// away three good bodies in four.
			if quoted[strings.ToLower(tok)] {
				if strings.Contains(tok, "/") && dirs[tok] {
					refs[tok] = true
				}
				continue
			}
			if strings.Contains(tok, "/") {
				if dirs[tok] {
					refs[tok] = true
					continue
				}
				// "loading/saving the config" is English, not a path. Only
				// something shaped like a repo path is held to being one.
				if pathLike(tok, dirs) {
					return fmt.Errorf("body-ungrounded: %q names %q, which is not in the staged changes", trim(line), tok)
				}
				continue
			}
			if identifierShaped(tok) && !vocab[strings.ToLower(tok)] {
				return fmt.Errorf("body-ungrounded: %q names %q, which is nowhere in the staged changes", trim(line), tok)
			}
		}
		if len(refs) == 0 {
			continue // nothing to attribute the symbols to
		}
		for _, tok := range tokens {
			own := owners[tok]
			if own == nil {
				continue // not a symbol from the spec; ordinary prose
			}
			// "which closes the AI" is English, not a reference to
			// appconfig.AI. An article in front means the word is being used
			// as a noun, and attributing it to a directory rejects a correct
			// bullet — which is how a good body got discarded one run in four.
			if articled(line, tok) {
				continue
			}
			if !ownedByAny(own, refs) {
				return fmt.Errorf("body-ungrounded: %q attributes %s to the wrong directory", trim(line), tok)
			}
		}
	}
	return nil
}

// pathLike reports whether a token containing a slash is actually a repo path
// rather than two words joined by one. A real path starts at a directory the
// change touched, or names a file by extension.
func pathLike(tok string, dirs map[string]bool) bool {
	if i := strings.IndexByte(tok, '/'); i > 0 && dirs[tok[:i]] {
		return true
	}
	base := tok[strings.LastIndexByte(tok, '/')+1:]
	if dot := strings.LastIndexByte(base, '.'); dot > 0 {
		switch strings.ToLower(base[dot:]) {
		case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".md", ".json", ".yaml", ".yml", ".sh":
			return true
		}
	}
	return false
}

// articled reports whether tok appears in line preceded by an article, which
// marks it as ordinary prose rather than a named identifier.
func articled(line, tok string) bool {
	low, lt := strings.ToLower(line), strings.ToLower(tok)
	for _, art := range []string{"the ", "a ", "an ", "its ", "their "} {
		if strings.Contains(low, art+lt) {
			return true
		}
	}
	return false
}

// ownedByAny reports whether a symbol's real home is one of the directories
// the bullet named, or beneath one of them.
func ownedByAny(owners, refs map[string]bool) bool {
	for owner := range owners {
		for ref := range refs {
			if owner == ref || strings.HasPrefix(owner, ref+"/") {
				return true
			}
		}
	}
	return false
}

// specTokens splits a bullet into candidate paths and identifiers, dropping
// the punctuation that prose wraps them in.
func specTokens(line string) []string {
	var out []string
	for _, field := range strings.Fields(line) {
		tok := strings.Trim(field, ",.;:()[]{}`\"'*")
		if tok != "" {
			out = append(out, tok)
		}
	}
	return out
}

func trim(line string) string {
	line = strings.TrimSpace(line)
	if len(line) > 60 {
		return line[:60] + "…"
	}
	return line
}
