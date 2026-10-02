package gitpanel

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/ui/theme"
)

// A commit's patch is shown as an accordion, like VS Code's commit view:
// one header row per file (click or ⏎ folds it) with that file's diff under
// it. The folded state is kept per path for the panel's lifetime.

type commitFile struct {
	path       string
	lines      []string // the diff --git section, header lines included
	adds, dels int
}

// fileHeadMark prefixes the synthetic detail line that stands for a file
// header; detailLines swaps it for the rendered accordion row.
const fileHeadMark = "\x00file "

// parseCommit splits `git show -p` output into the message header and the
// per-file sections.
func parseCommit(text string) (head []string, files []commitFile) {
	var cur *commitFile
	for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		switch {
		case strings.HasPrefix(l, "diff --git "):
			files = append(files, commitFile{path: diffPath(l)})
			cur = &files[len(files)-1]
		case cur == nil:
			head = append(head, l)
			continue
		case strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++"):
			cur.adds++
		case strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---"):
			cur.dels++
		}
		if cur != nil {
			cur.lines = append(cur.lines, l)
		}
	}
	for len(head) > 0 && strings.TrimSpace(head[len(head)-1]) == "" {
		head = head[:len(head)-1]
	}
	return head, files
}

// diffPath pulls the new-side path out of a `diff --git a/x b/x` line.
func diffPath(l string) string {
	if i := strings.Index(l, " b/"); i >= 0 {
		return l[i+3:]
	}
	return strings.TrimPrefix(l, "diff --git ")
}

// commitDetail loads a commit's patch into the detail pane as an accordion.
func (p *Panel) commitDetail(hash string) {
	p.commitHead, p.commitFiles = parseCommit(p.Repo.ShowPatch(hash))
	p.rebuildCommitDetail()
}

func (p *Panel) rebuildCommitDetail() {
	p.detail = append([]string{}, p.commitHead...)
	if len(p.commitFiles) > 0 {
		p.detail = append(p.detail, "")
	}
	for _, f := range p.commitFiles {
		p.detail = append(p.detail, fileHeadMark+f.path)
		if !p.foldedFiles[f.path] {
			p.detail = append(p.detail, f.lines[1:]...) // the diff --git line is the header itself
		}
	}
	p.detailCache = nil
}

// fileHeadAt is the path whose accordion header is on a rendered row, "".
func (p *Panel) fileHeadAt(row int) string {
	p.detailLines() // fills detailSrc
	if row < 0 || row >= len(p.detailSrc) {
		return ""
	}
	if l := p.detail[p.detailSrc[row][0]]; strings.HasPrefix(l, fileHeadMark) {
		return strings.TrimPrefix(l, fileHeadMark)
	}
	return ""
}

// toggleFile folds / unfolds one file of the commit, keeping the view where
// it is.
func (p *Panel) toggleFile(path string) {
	if p.foldedFiles == nil {
		p.foldedFiles = map[string]bool{}
	}
	p.foldedFiles[path] = !p.foldedFiles[path]
	p.rebuildCommitDetail()
	p.clamp()
}

// renderFileHead draws the accordion row for a file, w cells wide.
func (p *Panel) renderFileHead(path string, w int) string {
	arrow := "▾"
	if p.foldedFiles[path] {
		arrow = "▸"
	}
	var f commitFile
	for _, c := range p.commitFiles {
		if c.path == path {
			f = c
		}
	}
	stats := theme.DiffAddStyle.Render(fmt.Sprintf("+%d", f.adds)) + " " + theme.DiffDelStyle.Render(fmt.Sprintf("−%d", f.dels))
	head := lipgloss.NewStyle().Background(theme.HoverBg).Bold(true).Render(" " + arrow + " " + path + " ")
	return pad(ansi.Truncate(head, w-lipgloss.Width(stats)-1, "…")+" "+stats, w)
}

// decorateFileHeads swaps marker rows for rendered headers (continuation
// rows of a wrapped marker go blank).
func (p *Panel) decorateFileHeads(out []string, src [][]int, w int) {
	for i := range out {
		s := src[i][0]
		if s >= len(p.detail) || !strings.HasPrefix(p.detail[s], fileHeadMark) {
			continue
		}
		if i > 0 && src[i-1][0] == s {
			out[i] = pad("", w)
			continue
		}
		out[i] = p.renderFileHead(strings.TrimPrefix(p.detail[s], fileHeadMark), w)
	}
}

// graphHash is the commit on a `git log --graph --oneline` row ("" on a
// connector-only row).
func graphHash(l string) string {
	i := 0
	for i < len(l) && strings.ContainsRune("*|/\\ _-.", rune(l[i])) {
		i++
	}
	hash, _, _ := strings.Cut(l[i:], " ")
	return hash
}

// selectGraph shows the commit on graph row i in the detail pane. The list
// cursor's own detail comes back the next time it moves.
func (p *Panel) selectGraph(i int) {
	if i < 0 || i >= len(p.graph) {
		return
	}
	hash := graphHash(p.graph[i])
	if hash == "" {
		return
	}
	if p.Editing {
		p.stopEdit()
	}
	p.detailScroll, p.diffCursor = 0, 0
	p.selAnchor, p.selEnd = -1, -1
	p.hunkBtns = ""
	p.graphSel, p.graphSelHash = i, hash
	p.commitDetail(hash)
}
