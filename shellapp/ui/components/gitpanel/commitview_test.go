package gitpanel

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// A commit's detail is an accordion of files: a header row with +/− counts
// per file, folding on click or ⏎; the graph selects commits too.
func TestCommitAccordionAndGraphClick(t *testing.T) {
	repo, _ := lineRepo(t)
	p := New(repo)
	p.SetSize(140, 30)
	p.Section = SecCommits
	p.Refresh()
	if r := p.current(); r == nil || r.commit == nil {
		t.Fatal("cursor should be on the init commit")
	}
	view := func() string { return ansi.Strip(strings.Join(p.detailLines(), "\n")) }
	if v := view(); !strings.Contains(v, "▾ f.txt") || !strings.Contains(v, "+10 −0") || !strings.Contains(v, "+line x") {
		t.Fatalf("expanded accordion:\n%s", v)
	}
	// Find the header row and fold it from the keyboard.
	head := -1
	for i := range p.detailLines() {
		if p.fileHeadAt(i) == "f.txt" {
			head = i
		}
	}
	if head < 0 {
		t.Fatal("no file header row")
	}
	p.FocusDiff, p.diffCursor = true, head
	p.handleDiffKey(tea.KeyMsg{Type: tea.KeyEnter})
	if v := view(); !strings.Contains(v, "▸ f.txt") || strings.Contains(v, "+line x") {
		t.Fatalf("folded accordion:\n%s", v)
	}
	// A click on the header unfolds it again.
	p.HandleMouse(tea.MouseMsg{X: p.rightX() + 3, Y: listTop + head - p.detailScroll, Type: tea.MouseLeft, Action: tea.MouseActionPress})
	if v := view(); !strings.Contains(v, "▾ f.txt") {
		t.Fatalf("click should unfold:\n%s", v)
	}

	// Status tab: clicking the graph shows that commit; line ops are off
	// while the graph drives the detail pane, and the list takes over again
	// when the cursor moves.
	p.setSection(SecStatus)
	p.Refresh()
	if len(p.graph) == 0 {
		t.Fatal("no graph")
	}
	p.HandleMouse(tea.MouseMsg{X: 3, Y: p.ruleY() + 1, Type: tea.MouseLeft, Action: tea.MouseActionPress})
	if p.graphSel != 0 || !strings.Contains(view(), "▾ f.txt") || p.canPatch(p.current()) {
		t.Fatalf("graph click: sel=%d canPatch=%v\n%s", p.graphSel, p.canPatch(p.current()), view())
	}
	if !strings.Contains(ansi.Strip(strings.Join(p.leftColumn(), "\n")), graphHash(p.graph[0])) {
		t.Fatal("graph row missing")
	}
	p.move(1)
	if p.graphSel != -1 || strings.Contains(view(), "▾ f.txt") {
		t.Fatalf("list cursor should take the detail pane back: sel=%d", p.graphSel)
	}
}
