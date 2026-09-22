package texteditor

import "strings"

// Code folding by indentation, the way VS Code folds a file whose language has
// no folding provider: a line is a header when the next non-blank line is
// indented further, and its region runs to the last line still indented past
// it. Blank lines belong to whatever region surrounds them.

// foldRegion is a collapsible block: the header line, and the last line in it.
type foldRegion struct{ head, end int }

// indentOf measures a line's leading whitespace in display cells. A blank line
// has no indent of its own, so it reports -1 and inherits from its neighbours.
func indentOf(l string) int {
	w := 0
	for _, r := range l {
		switch r {
		case ' ':
			w++
		case '\t':
			w += 4 // matches theme.IndentWidth
		default:
			return w
		}
	}
	return -1 // blank
}

// foldRegions finds every collapsible block, outermost first.
func foldRegions(lines []string) []foldRegion {
	ind := make([]int, len(lines))
	for i, l := range lines {
		ind[i] = indentOf(l)
	}
	// next non-blank line at or after i
	nextReal := func(i int) int {
		for ; i < len(lines); i++ {
			if ind[i] >= 0 {
				return i
			}
		}
		return -1
	}

	var out []foldRegion
	for i := range lines {
		if ind[i] < 0 {
			continue
		}
		n := nextReal(i + 1)
		if n < 0 || ind[n] <= ind[i] {
			continue
		}
		// Run to the last line indented past the header, ignoring blanks so a
		// gap inside a block does not end it.
		end := i
		for j := n; j < len(lines); j++ {
			if ind[j] < 0 {
				continue
			}
			if ind[j] <= ind[i] {
				break
			}
			end = j
		}
		if end > i {
			out = append(out, foldRegion{i, end})
		}
	}
	return out
}

// foldState is the per-buffer folding, keyed by header line.
type foldState struct {
	collapsed map[int]bool
}

func (f *foldState) set(head int, on bool) {
	if f.collapsed == nil {
		f.collapsed = map[int]bool{}
	}
	if on {
		f.collapsed[head] = true
	} else {
		delete(f.collapsed, head)
	}
}

// hidden marks the lines currently folded away, and reports which headers are
// foldable so the gutter can draw its arrows. Regions whose header is no longer
// a header (the file was edited under them) are dropped.
func (f *foldState) hidden(lines []string) (hidden []bool, head map[int]foldRegion) {
	head = map[int]foldRegion{}
	hidden = make([]bool, len(lines))
	for _, r := range foldRegions(lines) {
		head[r.head] = r
	}
	for h := range f.collapsed {
		r, ok := head[h]
		if !ok {
			delete(f.collapsed, h) // the file changed underneath it
			continue
		}
		for i := r.head + 1; i <= r.end && i < len(hidden); i++ {
			hidden[i] = true
		}
	}
	return hidden, head
}

// foldSummary is the placeholder shown on a collapsed header, so the row still
// says what was hidden.
func foldSummary(lines []string, r foldRegion) string {
	n := r.end - r.head
	unit := "lines"
	if n == 1 {
		unit = "line"
	}
	// Close the block visually where it makes sense, as an editor would.
	tail := strings.TrimSpace(lines[r.end])
	if len(tail) > 0 && strings.ContainsAny(tail[:1], "}])") {
		return "⋯ " + tail
	}
	return "⋯ " + itoa(n) + " " + unit
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// ToggleFold collapses or expands the block at line, or the innermost block
// containing it — so the shortcut works from anywhere inside a function, not
// only on its first line.
func (t *TextEditor) ToggleFold(line int) {
	lines := strings.Split(t.TextArea.Value(), "\n")
	regions := foldRegions(lines)
	head := -1
	for _, r := range regions {
		if r.head == line {
			head = r.head
			break
		}
		// Innermost enclosing region wins, hence no early exit.
		if line > r.head && line <= r.end {
			head = r.head
		}
	}
	if head < 0 {
		t.status = "nothing to fold here"
		return
	}
	on := !t.cur.fold.collapsed[head]
	t.cur.fold.set(head, on)
	if on && t.TextArea.Line() > head {
		moveTo(&t.TextArea, head, 0) // the cursor must not sit inside a fold
	}
	t.followCursor()
	if on {
		t.status = "folded"
	} else {
		t.status = "unfolded"
	}
}

// FoldedCount reports how many blocks are collapsed (used by tests).
func (t *TextEditor) FoldedCount() int { return len(t.cur.fold.collapsed) }

// AnyFolded reports whether at least one block is currently collapsed, so a
// toggle can decide which way it should act.
func (t *TextEditor) AnyFolded() bool { return len(t.cur.fold.collapsed) > 0 }

// Foldable reports whether the buffer has any collapsible block at all.
func (t *TextEditor) Foldable() bool {
	return len(foldRegions(strings.Split(t.TextArea.Value(), "\n"))) > 0
}

// FoldAll collapses every block in the buffer. Nested regions are collapsed
// too, so expanding an outer block does not reveal a fully open inner one.
func (t *TextEditor) FoldAll() {
	lines := strings.Split(t.TextArea.Value(), "\n")
	regions := foldRegions(lines)
	if len(regions) == 0 {
		t.status = "nothing to fold"
		return
	}
	outermost := -1
	for _, r := range regions {
		t.cur.fold.set(r.head, true)
		if outermost < 0 {
			outermost = r.head
		}
	}
	// The cursor must not end up inside a fold; the first header is the
	// nearest visible line to wherever it was.
	if row := t.TextArea.Line(); row > outermost {
		hidden, _ := t.cur.fold.hidden(lines)
		if row < len(hidden) && hidden[row] {
			moveTo(&t.TextArea, outermost, 0)
		}
	}
	t.writeBack()
	t.followCursor()
	t.status = "folded all"
}

// UnfoldAll expands every collapsed block.
func (t *TextEditor) UnfoldAll() {
	if len(t.cur.fold.collapsed) == 0 {
		t.status = "nothing to expand"
		return
	}
	t.cur.fold.collapsed = map[int]bool{}
	t.writeBack()
	t.followCursor()
	t.status = "expanded all"
}

// ToggleFoldAll collapses everything, or expands everything when any block is
// already collapsed.
func (t *TextEditor) ToggleFoldAll() {
	if t.AnyFolded() {
		t.UnfoldAll()
		return
	}
	t.FoldAll()
}
