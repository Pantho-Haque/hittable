package gitx

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SubPatch cuts a unified diff down to the changed lines in [from, to] (indexes
// into the diff's lines, inclusive), the way `git add -p` builds a partial
// hunk. Every hunk that contributes nothing is dropped; hunk counts are
// recomputed.
//
// reverse says which side of the diff is on disk. Staging applies the patch
// forward to the index: an unselected "+" line is dropped and an unselected
// "-" line is still there, so it becomes context. Reverting or unstaging
// applies it in reverse against the post-image: there an unselected "+" line
// exists (context) and an unselected "-" line does not (dropped).
func SubPatch(diff string, from, to int, reverse bool) (string, bool) {
	if from > to {
		from, to = to, from
	}
	return SubPatchLines(diff, func(i int) bool { return i >= from && i <= to }, reverse)
}

// SubPatchLines is SubPatch for an arbitrary set of diff lines.
func SubPatchLines(diff string, selected func(i int) bool, reverse bool) (string, bool) {
	lines := strings.Split(strings.TrimRight(diff, "\n"), "\n")
	var out []string
	i := 0
	for i < len(lines) && !strings.HasPrefix(lines[i], "@@") {
		out = append(out, lines[i])
		i++
	}
	any := false
	for i < len(lines) {
		header := lines[i]
		i++
		var body []string
		picked, oldN, newN := false, 0, 0
		for i < len(lines) && !strings.HasPrefix(lines[i], "@@") {
			l := lines[i]
			sel := selected(i)
			i++
			switch {
			case l == "":
				continue
			case l[0] == '\\':
				body = append(body, l)
				continue
			case l[0] == ' ':
			case sel:
				picked = true
			case l[0] == '+' && !reverse, l[0] == '-' && reverse:
				continue // not chosen: never part of this patch
			default:
				l = " " + l[1:] // not chosen: still on disk, so plain context
			}
			if l[0] != '+' {
				oldN++
			}
			if l[0] != '-' {
				newN++
			}
			body = append(body, l)
		}
		if !picked {
			continue
		}
		any = true
		out = append(out, recount(header, oldN, newN))
		out = append(out, body...)
	}
	return strings.Join(out, "\n") + "\n", any
}

// recount rewrites a hunk header's line counts.
func recount(header string, oldN, newN int) string {
	var oldStart, newStart int
	rest := ""
	fmt.Sscanf(header, "@@ -%d", &oldStart)
	if i := strings.Index(header, "+"); i >= 0 {
		fmt.Sscanf(header[i:], "+%d", &newStart)
	}
	if i := strings.Index(header[2:], "@@"); i >= 0 {
		rest = header[i+4:]
	}
	return "@@ -" + strconv.Itoa(oldStart) + "," + strconv.Itoa(oldN) +
		" +" + strconv.Itoa(newStart) + "," + strconv.Itoa(newN) + " @@" + rest
}

// Apply runs `git apply` on a patch: cached targets the index, reverse
// undoes the patch. Combined they stage, unstage or discard a set of lines.
func (r *Repo) Apply(patch string, cached, reverse bool) error {
	f, err := os.CreateTemp("", "hittable-*.patch")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(patch); err != nil {
		f.Close()
		return err
	}
	f.Close()
	// --unidiff-zero: hunks built from the editor buffer carry no context.
	args := []string{"apply", "--recount", "--whitespace=nowarn", "--unidiff-zero"}
	if cached {
		args = append(args, "--cached")
	}
	if reverse {
		args = append(args, "--reverse")
	}
	_, err = r.Run(append(args, f.Name())...)
	return err
}

// Graph is `git log --graph --oneline` for a ref, one row per line.
func (r *Repo) Graph(ref string, n int) ([]string, error) {
	args := []string{"log", "--graph", "--oneline", "--decorate=short", "-n", strconv.Itoa(n)}
	if ref != "" {
		args = append(args, ref)
	}
	out, err := r.Run(args...)
	if err != nil {
		return nil, err
	}
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}
