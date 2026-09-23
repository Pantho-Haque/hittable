// Package commitmsg turns the staged changes of a repository into a commit
// message. Collect reads git through a one-method interface, Pack renders the
// result as prompt input, Generate writes a conventional-commit message from
// it mechanically, and Validate/Repair check a message against commitlint's
// rules.
//
// Non-goals: this package never runs git itself (the caller supplies the
// Runner), never touches the network, and never reads a working-tree file —
// everything it knows comes from git's own machine-readable output.
package commitmsg

import (
	"strconv"
	"strings"
)

// Runner executes a git subcommand in a repository and returns stdout.
// *gitx.Repo satisfies it structurally, which is what keeps this package free
// of an import cycle and lets tests fake git in three lines.
type Runner interface {
	Run(args ...string) (string, error)
}

// Bounds applied while collecting, so a huge staged change can never turn one
// keypress into seconds of string work.
const (
	maxFiles        = 200      // files carried in the digest
	maxRawDiff      = 1 << 20  // total staged diff we are willing to parse
	maxFileDiff     = 64 << 10 // per file; past this the file is counts-only
	maxHunksPerFile = 3
	maxHunkLines    = 40
	diffContext     = 3
	historySample   = 200 // subjects sampled to learn the repo's convention
	docMaxLines     = 3   // lines of a package doc comment kept
	docMaxBytes     = 200
)

// Change is one staged path.
type Change struct {
	Path, OldPath  string // OldPath set on rename/copy
	Status         byte   // 'A' 'M' 'D' 'R' 'C'
	Added, Removed int
	Binary         bool
	Hunks          []string // bounded; empty when demoted to counts-only
	Symbols        []string // new exported symbols, in declaration order
	Doc            string   // the package doc comment, when this change adds one
}

// Digest is the shared intermediate: the heuristic and a model consume
// byte-identical input, so the fallback is the same code path rather than a
// parallel one.
type Digest struct {
	Branch     string
	Prefix     string // repo-relative path of the working root, e.g. "shellapp/"
	Files      []Change
	Truncated  bool
	Convention bool     // >=50% of sampled history matches the conventional pattern
	Scopes     []string // learned from history, most frequent first
}

// Collect reads the staged changes and a sample of history. An error means
// git itself failed on the staged diff; a repository with no commits yet is
// not an error, the history fields are simply empty.
func Collect(r Runner) (*Digest, error) {
	// Convention holds until the history contradicts it, so the first commit
	// in a fresh repository still gets a type.
	d := &Digest{Convention: true}
	d.Branch = collectBranch(r)
	if out, err := r.Run("rev-parse", "--show-prefix"); err == nil {
		d.Prefix = strings.TrimSpace(out)
	}

	ns, err := r.Run("diff", "--cached", "--name-status", "-M", "-C")
	if err != nil {
		return nil, err
	}
	files := parseNameStatus(ns)
	if len(files) > maxFiles {
		files, d.Truncated = files[:maxFiles], true
	}
	if num, err := r.Run("diff", "--cached", "--numstat", "-M", "-C"); err == nil {
		applyNumstat(files, parseNumstat(num))
	}
	d.Files = files

	collectHunks(r, d)
	collectHistory(r, d)
	return d, nil
}

func collectBranch(r Runner) string {
	if out, err := r.Run("branch", "--show-current"); err == nil {
		if b := strings.TrimSpace(out); b != "" {
			return b
		}
	}
	if out, err := r.Run("rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		return strings.TrimSpace(out)
	}
	return ""
}

// parseNameStatus reads "<status>\t<path>" lines, with a second tab-separated
// field carrying the destination of a rename or copy.
func parseNameStatus(out string) []Change {
	var cs []Change
	for _, line := range splitLines(out) {
		f := strings.Split(line, "\t")
		if len(f) < 2 || f[0] == "" {
			continue
		}
		c := Change{Status: f[0][0], Path: f[1]}
		if (c.Status == 'R' || c.Status == 'C') && len(f) >= 3 {
			c.OldPath, c.Path = f[1], f[2]
		}
		cs = append(cs, c)
	}
	return cs
}

type counts struct {
	added, removed int
	binary         bool
}

// parseNumstat keeps the numstat lines in git's own order. The path field is
// ambiguous for renames (git rewrites it as "old => new" without -z), so the
// counts are zipped positionally onto the name-status entries, which come from
// the same diff with the same options and therefore in the same order.
func parseNumstat(out string) []counts {
	var cs []counts
	for _, line := range splitLines(out) {
		f := strings.SplitN(line, "\t", 3)
		if len(f) < 3 {
			continue
		}
		if f[0] == "-" && f[1] == "-" {
			cs = append(cs, counts{binary: true})
			continue
		}
		a, _ := strconv.Atoi(f[0])
		rm, _ := strconv.Atoi(f[1])
		cs = append(cs, counts{added: a, removed: rm})
	}
	return cs
}

func applyNumstat(files []Change, cs []counts) {
	for i := range files {
		if i >= len(cs) {
			return
		}
		files[i].Added, files[i].Removed, files[i].Binary = cs[i].added, cs[i].removed, cs[i].binary
	}
}

// collectHunks fills in bounded hunks for every file worth reading. Binaries
// and secrets are never read at all.
func collectHunks(r Runner, d *Digest) {
	want := false
	for _, c := range d.Files {
		if readable(c) {
			want = true
			break
		}
	}
	if !want {
		return
	}
	out, err := r.Run("diff", "--cached", "-M", "-C", "--unified="+strconv.Itoa(diffContext))
	if err != nil {
		return
	}
	if len(out) > maxRawDiff {
		d.Truncated = true
		return
	}
	sections := splitSections(out)
	for i := range d.Files {
		if !readable(d.Files[i]) {
			continue
		}
		body, ok := sections[d.Files[i].Path]
		if !ok {
			continue
		}
		if len(body) > maxFileDiff {
			d.Truncated = true
			continue
		}
		// Symbols are scanned before the hunks are bounded: a newly added
		// 600-line file is one hunk, and trimming it to head and tail would
		// leave the import block and the last declaration as the only things
		// the subject could be named after.
		added := strings.Split(body, "\n")
		d.Files[i].Symbols = scanSymbols(added, d.Files[i].Path)
		d.Files[i].Doc = scanDoc(added)
		hunks, dropped := boundHunks(body)
		d.Files[i].Hunks = hunks
		if dropped {
			d.Truncated = true
		}
	}
}

func readable(c Change) bool { return !c.Binary && !IsSecret(c.Path) }

// splitSections maps the post-image path of each "diff --git" section to its
// raw text.
func splitSections(diff string) map[string]string {
	sections := map[string]string{}
	var path string
	var buf []string
	flush := func() {
		if path != "" {
			sections[path] = strings.Join(buf, "\n")
		}
		path, buf = "", nil
	}
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "+++ b/"):
			path = strings.TrimPrefix(line, "+++ b/")
			continue
		case strings.HasPrefix(line, "--- a/") && path == "":
			// Deletion: the post-image is /dev/null, so name the section
			// after the pre-image instead.
			path = strings.TrimPrefix(line, "--- a/")
			continue
		case strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- "):
			continue
		}
		if path != "" {
			buf = append(buf, line)
		}
	}
	flush()
	return sections
}

// scanDoc pulls the package doc comment out of the added lines. It is the one
// piece of semantic content in a diff that a model could never infer from
// identifiers — what the package is for, in the author's own words — and it is
// true by construction, because the author wrote it in this very change.
func scanDoc(lines []string) string {
	var parts []string
	for _, line := range lines {
		text := strings.TrimSpace(strings.TrimPrefix(line, "+"))
		if len(parts) == 0 {
			if !strings.HasPrefix(text, "// Package ") {
				continue
			}
			parts = append(parts, strings.TrimPrefix(text, "// "))
		} else {
			// Keep gathering while the sentence is unfinished: a doc comment
			// wraps, and half of one reads as a mistake.
			if !strings.HasPrefix(text, "//") || len(parts) >= docMaxLines {
				break
			}
			parts = append(parts, strings.TrimSpace(strings.TrimPrefix(text, "//")))
		}
		if strings.HasSuffix(parts[len(parts)-1], ".") {
			break
		}
	}
	doc := strings.TrimSpace(strings.Join(parts, " "))
	if i := strings.Index(doc, ". "); i >= 0 {
		doc = doc[:i+1]
	}
	if len(doc) > docMaxBytes {
		doc = doc[:docMaxBytes]
		if i := strings.LastIndexByte(doc, ' '); i > 0 {
			doc = doc[:i]
		}
	}
	return doc
}

// boundHunks cuts a file's diff body down to at most maxHunksPerFile hunks of
// at most maxHunkLines lines each, keeping both ends of a long hunk so the
// shape of the change survives. It reports whether anything was dropped.
func boundHunks(body string) ([]string, bool) {
	var hunks []string
	var cur []string
	dropped := false
	flush := func() {
		if len(cur) == 0 {
			return
		}
		if len(hunks) < maxHunksPerFile {
			hunks = append(hunks, strings.Join(trimHunk(cur, &dropped), "\n"))
		} else {
			dropped = true
		}
		cur = nil
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "@@") {
			flush()
		}
		if len(cur) > 0 || strings.HasPrefix(line, "@@") {
			cur = append(cur, line)
		}
	}
	flush()
	return hunks, dropped
}

func trimHunk(lines []string, dropped *bool) []string {
	if len(lines) <= maxHunkLines {
		return lines
	}
	*dropped = true
	head := maxHunkLines / 2
	tail := maxHunkLines - head - 1
	out := append([]string{}, lines[:head]...)
	out = append(out, "…")
	return append(out, lines[len(lines)-tail:]...)
}

// collectHistory learns the repository's conventions from its subjects. The
// commits themselves are not kept: the prompt teaches its shape with one
// invented example, because this repository's own messages were written from
// diffs rather than from the specification the model is given.
func collectHistory(r Runner, d *Digest) {
	if out, err := r.Run("log", "-n", strconv.Itoa(historySample), "--pretty=%s"); err == nil {
		d.Convention, d.Scopes = learn(splitLines(out))
	}
}

// learn reports whether the sampled history follows conventional commits and
// which scopes it uses, most frequent first.
func learn(subjects []string) (bool, []string) {
	var conventional int
	count := map[string]int{}
	var order []string
	for _, s := range subjects {
		m := headerRe.FindStringSubmatch(strings.TrimSpace(s))
		if m == nil {
			continue
		}
		conventional++
		if scope := m[2]; scope != "" {
			if count[scope] == 0 {
				order = append(order, scope)
			}
			count[scope]++
		}
	}
	// Sort by frequency, ties broken by first appearance, so the result is
	// stable for a given history.
	sortStable(order, func(a, b string) bool { return count[a] > count[b] })
	// A repository with no history yet has no counter-evidence, so the
	// convention holds vacuously: a first commit gets "feat: …" rather than a
	// bare subject.
	return conventional*2 >= len(subjects), order
}

func splitLines(out string) []string {
	out = strings.Trim(out, "\n")
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// sortStable is an insertion sort: the slices here are tiny and this keeps the
// tie-breaking by original position explicit.
func sortStable(s []string, less func(a, b string) bool) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && less(s[j], s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
