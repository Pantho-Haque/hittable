package gitpanel

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hittable/shellapp/internal/gitx"
)

// Line-level stage / unstage / revert, VS Code style: the diff pane has a
// cursor and a selection, and an op applies a sub-patch built from just
// those lines (or the cursor's whole hunk when nothing is selected).

// canPatch reports whether the row's diff can be applied by line: a tracked,
// non-conflicting file, diffed without -w (a -w diff does not apply).
func (p *Panel) canPatch(r *row) bool {
	return r != nil && r.file != nil && !r.file.Conflict() && !r.file.Untracked() && !p.IgnoreWS &&
		r.file.Index != 'D' && r.file.Worktree != 'D'
}

// hunkOf returns the rendered-row span of the hunk containing row.
func (p *Panel) hunkOf(row int) (int, int) {
	dl := p.detailLines()
	isHead := func(i int) bool {
		s := p.detailSrc[i][0]
		return strings.HasPrefix(p.detail[s], "@@") && (i == 0 || p.detailSrc[i-1][0] != s)
	}
	a := row
	for a > 0 && !isHead(a) {
		a--
	}
	b := row
	for b+1 < len(dl) && !isHead(b+1) {
		b++
	}
	return a, b
}

// opLines is the set of diff lines an op acts on: the selection, else the
// hunk under the cursor (or the given row).
func (p *Panel) opLines(row int) map[int]bool {
	a, b := p.selRange()
	if a < 0 {
		a, b = p.hunkOf(row)
	}
	set := map[int]bool{}
	for i := a; i <= b && i < len(p.detailSrc); i++ {
		for _, s := range p.detailSrc[i] {
			set[s] = true
		}
	}
	return set
}

// lineOp stages, unstages or reverts the chosen lines. Revert destroys work,
// so it is confirmed; the others are cheap to undo and run at once.
func (p *Panel) lineOp(op string, row int) {
	r := p.current()
	if !p.canPatch(r) {
		p.Message = "line ops need a tracked file diffed without -w"
		return
	}
	if op == "stage" && r.staged {
		p.Message = "already staged"
		return
	}
	if len(p.detail) == 0 || len(p.detailLines()) == 0 { // detailLines fills detailSrc
		return
	}
	set := p.opLines(row)
	diff := strings.Join(p.detail, "\n") + "\n"
	reverse := op != "stage"
	patch, ok := gitx.SubPatchLines(diff, func(i int) bool { return set[i] }, reverse)
	if !ok {
		p.Message = "no changed lines here"
		return
	}
	cached := op != "revert"
	apply := func() {
		// The refresh after the op rebuilds the diff and resets the cursor;
		// put it back near where the user was.
		cur, scroll := p.diffCursor, p.detailScroll
		p.run(op+" lines", func() error { return p.Repo.Apply(patch, cached, reverse) })
		p.detailScroll = scroll
		p.setDiffCursor(cur)
	}
	if op == "revert" {
		p.confirm("revert these lines in "+r.file.Path, apply)
		return
	}
	apply()
}

// diffOp maps the undo button / u key to the op that fits the diff shown.
func (p *Panel) diffOp(btn string) string {
	if btn == "stage" {
		return "stage"
	}
	if r := p.current(); r != nil && r.staged {
		return "unstage"
	}
	return "revert"
}

// setDiffCursor moves the cursor and keeps it on screen.
func (p *Panel) setDiffCursor(row int) {
	n := len(p.detailLines())
	if n == 0 {
		return
	}
	p.diffCursor = min(max(row, 0), n-1)
	if p.diffCursor < p.detailScroll {
		p.detailScroll = p.diffCursor
	}
	if p.diffCursor >= p.detailScroll+p.detailRows() {
		p.detailScroll = p.diffCursor - p.detailRows() + 1
	}
	p.clamp()
}

// handleDiffKey routes keys while the diff pane has the keyboard. Returns
// false for keys it does not own, so section switching keeps working.
func (p *Panel) handleDiffKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "esc", "tab":
		p.FocusDiff = false
	case "up", "k":
		p.selAnchor, p.selEnd = -1, -1
		p.setDiffCursor(p.diffCursor - 1)
	case "down", "j":
		p.selAnchor, p.selEnd = -1, -1
		p.setDiffCursor(p.diffCursor + 1)
	case "shift+up", "shift+down":
		if p.selAnchor < 0 {
			p.selAnchor = p.diffCursor
		}
		d := 1
		if msg.String() == "shift+up" {
			d = -1
		}
		p.setDiffCursor(p.diffCursor + d)
		p.selEnd = p.diffCursor
	case "pgup", "ctrl+u":
		p.setDiffCursor(p.diffCursor - (p.detailRows() - 1))
	case "pgdown", "ctrl+d", " ":
		p.setDiffCursor(p.diffCursor + (p.detailRows() - 1))
	case "g", "home":
		p.setDiffCursor(0)
	case "G", "end":
		p.setDiffCursor(len(p.detailLines()))
	case "s", "+":
		p.lineOp("stage", p.diffCursor)
	case "u", "-":
		p.lineOp(p.diffOp("undo"), p.diffCursor)
	case "d", "x":
		if r := p.current(); r != nil && !r.staged {
			p.lineOp("revert", p.diffCursor)
		}
	default:
		return false
	}
	return true
}
