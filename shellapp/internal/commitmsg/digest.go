package commitmsg

import (
	"path"
	"strconv"
	"strings"
)

// DefaultBudget is the byte budget Pack is sized against: roughly 3-4k tokens,
// leaving room for a system prompt, few-shot examples and the answer.
const DefaultBudget = 12000

// Paths whose diffs are generated or vendored: listed, but the last to have
// their contents packed.
var demotedPaths = []string{
	"node_modules/", "vendor/", "dist/", "build/", ".next/", "testdata/", "target/",
}

var demotedNames = []string{
	"go.sum", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lockb",
	"composer.lock", "Cargo.lock", "poetry.lock",
}

// Files whose contents must never reach a prompt. Everything here is listed by
// name with its line counts and nothing else: the packer is the exact path
// that would ship a diff to a remote provider the day one is added.
func IsSecret(p string) bool {
	base := strings.ToLower(path.Base(p))
	switch {
	case strings.HasPrefix(base, ".env"),
		strings.HasPrefix(base, "id_rsa"),
		strings.HasSuffix(base, ".pem"),
		strings.HasSuffix(base, ".key"),
		strings.Contains(base, "credential"),
		strings.Contains(base, "secret"):
		return true
	}
	return false
}

func isDemoted(p string) bool {
	lower := strings.ToLower(p)
	for _, d := range demotedPaths {
		if strings.Contains(lower, d) {
			return true
		}
	}
	base := path.Base(p)
	for _, n := range demotedNames {
		if base == n {
			return true
		}
	}
	return strings.HasSuffix(base, ".min.js") || strings.HasSuffix(base, ".pb.go") ||
		strings.HasSuffix(base, ".lock")
}

// rank scores a file for packing. Higher wins: generated files sink, files
// under the working root and files declaring new exported symbols rise.
func (d *Digest) rank(c Change) int {
	score := c.Added + c.Removed
	if score > 500 {
		score = 500
	}
	if isDemoted(c.Path) {
		score -= 2000
	}
	if d.Prefix != "" && strings.HasPrefix(c.Path, d.Prefix) {
		score += 1000
	}
	if len(exportedSymbols(c)) > 0 {
		score += 400
	}
	return score
}

// ranked returns the file indexes ordered best first, ties broken by the order
// git listed them so packing is deterministic.
func (d *Digest) ranked() []int {
	idx := make([]int, len(d.Files))
	scores := make([]int, len(d.Files))
	for i := range d.Files {
		idx[i], scores[i] = i, d.rank(d.Files[i])
	}
	for i := 1; i < len(idx); i++ {
		for j := i; j > 0 && scores[idx[j]] > scores[idx[j-1]]; j-- {
			idx[j], idx[j-1] = idx[j-1], idx[j]
		}
	}
	return idx
}

// Pack renders the digest as prompt input, no larger than budget bytes.
//
// Survival order: the branch and the full file list with status letters and
// counts come first and are the last thing dropped. Per-file hunks follow,
// best-ranked first, degraded down a ladder until the whole thing fits:
// context lines go first, then trailing hunks, then whole low-rank files, then
// everything but the file list. Pack sets Truncated whenever it degrades, so
// the prompt can say the diff was cut rather than let the model invent.
func (d *Digest) Pack(budget int) string {
	if budget <= 0 {
		budget = DefaultBudget
	}
	order := d.ranked()

	for level := 0; level <= 2; level++ {
		if out, ok := d.assemble(order, level, len(order), budget); ok {
			return out
		}
	}
	// Still over: drop whole files from the worst-ranked up.
	for keep := len(order) - 1; keep > 0; keep-- {
		if out, ok := d.assemble(order, 2, keep, budget); ok {
			d.Truncated = true
			return out
		}
	}
	d.Truncated = true
	out, _ := d.assemble(order, 2, 0, budget)
	return clamp(out, budget)
}

// clamp is the backstop for a budget too small even for the file list: cut at
// a line boundary so the caller's promise about the size always holds.
func clamp(s string, budget int) string {
	if len(s) <= budget {
		return s
	}
	s = s[:budget]
	if i := strings.LastIndexByte(s, '\n'); i > 0 {
		s = s[:i+1]
	}
	return s
}

// assemble builds the packed text at a degradation level, packing hunks for at
// most keep files, and reports whether it fits the budget. Level 0 is the full
// collected hunks, level 1 drops context lines (what --unified=0 would have
// produced), level 2 keeps only the first hunk of each file.
func (d *Digest) assemble(order []int, level, keep, budget int) (string, bool) {
	if level > 0 || keep < len(order) {
		d.Truncated = true
	}
	var b strings.Builder
	b.WriteString(d.header(budget))
	if b.Len() > budget {
		return b.String(), false
	}
	for n, i := range order {
		if n >= keep {
			break
		}
		c := d.Files[i]
		body := packHunks(c, level)
		if body == "" {
			continue
		}
		section := "\n--- " + c.Path + "\n" + body + "\n"
		if b.Len()+len(section) > budget {
			return b.String(), false
		}
		b.WriteString(section)
	}
	return b.String(), true
}

// header is the branch plus the file list. It is the one part that must always
// fit, so past the budget the list itself is cut short and counted.
func (d *Digest) header(budget int) string {
	var b strings.Builder
	if d.Branch != "" {
		b.WriteString("branch: " + d.Branch + "\n")
	}
	if d.Prefix != "" {
		b.WriteString("working directory: " + d.Prefix + "\n")
	}
	b.WriteString("staged files (" + strconv.Itoa(len(d.Files)) + "):\n")
	limit := budget * 3 / 4
	for i, c := range d.Files {
		line := fileLine(c)
		if b.Len()+len(line) > limit {
			d.Truncated = true
			b.WriteString("… and " + strconv.Itoa(len(d.Files)-i) + " more files\n")
			break
		}
		b.WriteString(line)
	}
	if d.Truncated {
		b.WriteString("note: the staged diff was truncated to fit\n")
	}
	return b.String()
}

func fileLine(c Change) string {
	var b strings.Builder
	b.WriteByte(c.Status)
	b.WriteByte(' ')
	if c.OldPath != "" {
		b.WriteString(c.OldPath + " -> ")
	}
	b.WriteString(c.Path)
	switch {
	case c.Binary:
		b.WriteString(" (binary)")
	case IsSecret(c.Path):
		b.WriteString(" (+" + strconv.Itoa(c.Added) + " -" + strconv.Itoa(c.Removed) + ", contents withheld)")
	default:
		b.WriteString(" (+" + strconv.Itoa(c.Added) + " -" + strconv.Itoa(c.Removed) + ")")
	}
	b.WriteByte('\n')
	return b.String()
}

// packHunks renders one file's hunks at a degradation level. A binary or a
// secret never contributes content, whatever the digest happens to carry.
func packHunks(c Change, level int) string {
	if c.Binary || IsSecret(c.Path) || len(c.Hunks) == 0 {
		return ""
	}
	hunks := c.Hunks
	if level >= 2 && len(hunks) > 1 {
		hunks = hunks[:1]
	}
	var out []string
	for _, h := range hunks {
		if level >= 1 {
			h = dropContext(h)
		}
		if strings.TrimSpace(h) != "" {
			out = append(out, h)
		}
	}
	return strings.Join(out, "\n")
}

// dropContext removes unchanged lines, which is what git would have produced
// with --unified=0 and is the cheapest thing to give up.
func dropContext(hunk string) string {
	var out []string
	for _, line := range strings.Split(hunk, "\n") {
		if line == "" || strings.HasPrefix(line, " ") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
