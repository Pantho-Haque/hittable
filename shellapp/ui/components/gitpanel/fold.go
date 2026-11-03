package gitpanel

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/ui/theme"
)

// A working-copy diff is loaded with the whole file as context, then split
// back into one hunk per change (regroup), so the hunk buttons and line ops
// still act on one change at a time. Long runs of unchanged lines are drawn
// as one fold row ("⋯ 42 unchanged lines"); a click or ⏎ opens one, F shows
// or folds the whole file.

// foldMargin is how many unchanged lines stay visible beside a change, as git
// shows by default.
const foldMargin = 3

// minFold is the smallest run worth a fold row: hiding fewer lines saves nothing.
const minFold = 3

// foldMark starts the stand-in line a fold row is rendered from; it carries
// the number of lines it hides. No diff line starts with a NUL.
const foldMark = "\x00fold:"

func isChange(l string) bool {
	return (strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++")) ||
		(strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---")) ||
		strings.HasPrefix(l, "\\")
}

// regroup splits every hunk of a full-context diff into one hunk per group of
// changes (changes closer than 2*margin unchanged lines stay together). Each
// new hunk starts margin lines above its first change; the first one keeps
// the lines above it and the last one the lines below, so every line is still
// in exactly one hunk and any hunk applies on its own.
func regroup(diff string, margin int) string {
	lines := strings.Split(diff, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		if !strings.HasPrefix(lines[i], "@@") {
			out = append(out, lines[i])
			i++
			continue
		}
		header := lines[i]
		i++
		start := i
		for i < len(lines) && !strings.HasPrefix(lines[i], "@@") && !strings.HasPrefix(lines[i], "diff ") {
			i++
		}
		out = append(out, splitHunk(header, lines[start:i], margin)...)
	}
	return strings.Join(out, "\n")
}

func splitHunk(header string, body []string, margin int) []string {
	var oldStart, newStart int
	fmt.Sscanf(header, "@@ -%d", &oldStart)
	if j := strings.Index(header, "+"); j >= 0 {
		fmt.Sscanf(header[j:], "+%d", &newStart)
	}
	// Where each change group starts (an index into body).
	var starts []int
	last := -1 // index of the last change line seen
	for k, l := range body {
		if !isChange(l) {
			continue
		}
		if last < 0 || k-last-1 > 2*margin {
			starts = append(starts, k)
		}
		last = k
	}
	if len(starts) < 2 {
		return append([]string{header}, body...)
	}
	// The old / new line number each body line starts at.
	oldAt, newAt := make([]int, len(body)+1), make([]int, len(body)+1)
	o, n := oldStart, newStart
	for k, l := range body {
		oldAt[k], newAt[k] = o, n
		switch {
		case l == "", strings.HasPrefix(l, "\\"):
		case l[0] == '-':
			o++
		case l[0] == '+':
			n++
		default:
			o, n = o+1, n+1
		}
	}
	oldAt[len(body)], newAt[len(body)] = o, n
	// A trailing "" (the diff's final newline) belongs after every hunk.
	end := len(body)
	for end > 0 && body[end-1] == "" {
		end--
	}
	_, rest, _ := strings.Cut(strings.TrimPrefix(header, "@@"), "@@")
	var out []string
	for g, s := range starts {
		from := 0
		if g > 0 {
			from = s - margin
		}
		to := end
		if g+1 < len(starts) {
			to = starts[g+1] - margin
		}
		h := "@@ -" + strconv.Itoa(oldAt[from]) + "," + strconv.Itoa(oldAt[to]-oldAt[from]) +
			" +" + strconv.Itoa(newAt[from]) + "," + strconv.Itoa(newAt[to]-newAt[from]) + " @@"
		if g == 0 {
			h += rest
		}
		out = append(out, h)
		out = append(out, body[from:to]...)
	}
	return append(out, body[end:]...)
}

// foldRun is a run of unchanged detail lines that can be hidden.
type foldRun struct{ from, to int } // inclusive

// foldRuns lists the runs of unchanged lines inside hunks that are long
// enough to fold, keeping foldMargin lines next to each change.
func foldRuns(detail []string) []foldRun {
	var runs []foldRun
	inHunk := false
	for i := 0; i < len(detail); i++ {
		l := detail[i]
		if strings.HasPrefix(l, "@@") {
			inHunk = true
			continue
		}
		if strings.HasPrefix(l, "diff ") {
			inHunk = false
		}
		if !inHunk || !strings.HasPrefix(l, " ") {
			continue
		}
		a := i
		for i+1 < len(detail) && strings.HasPrefix(detail[i+1], " ") {
			i++
		}
		b := i
		if a > 0 && isChange(detail[a-1]) {
			a += foldMargin
		}
		if b+1 < len(detail) && isChange(detail[b+1]) {
			b -= foldMargin
		}
		if b-a+1 >= minFold {
			runs = append(runs, foldRun{a, b})
		}
	}
	return runs
}

// visibleDetail is the detail with each closed fold replaced by one stand-in
// line, and the detail index each visible line came from.
func (p *Panel) visibleDetail() ([]string, []int) {
	vis, src := make([]string, 0, len(p.detail)), make([]int, 0, len(p.detail))
	runs := map[int]int{}
	if !p.FullFile {
		for _, r := range foldRuns(p.detail) {
			if !p.unfolded[r.from] {
				runs[r.from] = r.to
			}
		}
	}
	for i := 0; i < len(p.detail); i++ {
		if to, ok := runs[i]; ok {
			vis, src = append(vis, foldMark+strconv.Itoa(to-i+1)), append(src, i)
			i = to
			continue
		}
		vis, src = append(vis, p.detail[i]), append(src, i)
	}
	return vis, src
}

// foldCount is the number of lines a stand-in line hides (0 for any other line).
func foldCount(l string) int {
	if !strings.HasPrefix(l, foldMark) {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimPrefix(l, foldMark))
	return n
}

// renderFold draws a fold row across the pane.
func renderFold(n, w int) string {
	label := fmt.Sprintf("  ⋯  %d unchanged lines", n) // n >= minFold
	if hint := "   click or ⏎ to show · F whole file"; lipgloss.Width(label+hint) <= w {
		label += hint
	}
	return foldStyle.Render(pad(ansi.Truncate(label, w, "…"), w))
}

var foldStyle = lipgloss.NewStyle().Foreground(theme.SubtextColor).Background(theme.SurfaceColor)

// foldAt is the detail index of the fold row drawn on a rendered row, or -1.
func (p *Panel) foldAt(row int) int {
	p.detailLines() // fills detailFolds
	if f, ok := p.detailFolds[row]; ok {
		return f
	}
	return -1
}

// unfold shows the lines of one fold.
func (p *Panel) unfold(from int) {
	if p.unfolded == nil {
		p.unfolded = map[int]bool{}
	}
	p.unfolded[from] = true
	p.foldGen++
	p.clamp()
}

// ToggleFullFile switches between the whole file and only the changes (with
// every fold closed again), keeping the line at the top of the pane in view.
func (p *Panel) ToggleFullFile() {
	top := -1
	if dl := p.detailLines(); p.detailScroll < len(dl) {
		top = p.detailSrc[p.detailScroll][0]
	}
	p.FullFile = !p.FullFile
	p.unfolded = nil
	p.foldGen++
	p.Message = "showing only the changes"
	if p.FullFile {
		p.Message = "showing the whole file"
	}
	if top < 0 {
		return
	}
	p.detailLines()
	for i, s := range p.detailSrc {
		if s[0] >= top {
			p.detailScroll = i
			break
		}
	}
	p.setDiffCursor(max(p.diffCursor, p.detailScroll))
	p.clamp()
}

// unifiedRows renders diff lines one under the other (wrapped or clipped to
// w), each added / removed row tinted to the edge, and maps each row to the
// line it shows.
func unifiedRows(lines []string, w int, wrap bool) ([]string, [][]int) {
	var out []string
	var src [][]int
	for i, l := range lines {
		var chunks []string
		if n := foldCount(l); n > 0 {
			chunks = []string{renderFold(n, w)}
		} else {
			chunks = fullWidth(diffLine(showWhitespace(l)), w, wrap)
			if st, tinted := diffRowStyle(l); tinted {
				// Fill the rest of the row so the tint reaches the edge.
				for j, c := range chunks {
					if n := w - lipgloss.Width(c); n > 0 {
						chunks[j] = c + st.Render(strings.Repeat(" ", n))
					}
				}
			}
		}
		out = append(out, chunks...)
		for range chunks {
			src = append(src, []int{i})
		}
	}
	return out, src
}
