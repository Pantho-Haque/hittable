package gitpanel

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/internal/gitx"
)

// A 40-line file with lines 5 and 35 changed: the diff covers the whole file,
// one hunk per change, with the 26 unchanged lines between them folded.
func foldRepo(t *testing.T) (*gitx.Repo, string, *Panel) {
	t.Helper()
	r, dir := flowRepo(t)
	var lines []string
	for i := 1; i <= 40; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	writeFile(t, dir, "long.txt", strings.Join(lines, "\n")+"\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "long")
	lines[4], lines[34] = "FIVE", "THIRTY-FIVE"
	writeFile(t, dir, "long.txt", strings.Join(lines, "\n")+"\n")
	p := flowPanel(t, r)
	selectFile(t, p, "long.txt", false)
	return r, dir, p
}

func plainRows(p *Panel) []string {
	var out []string
	for _, l := range p.detailLines() {
		out = append(out, strings.TrimRight(ansi.Strip(l), " "))
	}
	return out
}

func foldRows(p *Panel) []string {
	var out []string
	for _, l := range plainRows(p) {
		if strings.Contains(l, "unchanged line") {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

func TestRegroupSplitsTheWholeFileIntoOneHunkPerChange(t *testing.T) {
	_, _, p := foldRepo(t)
	var heads []string
	n := 0
	for _, l := range p.detail {
		if strings.HasPrefix(l, "@@") {
			heads = append(heads, strings.TrimSpace(l))
		}
		if strings.HasPrefix(l, " ") || strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++") {
			n++
		}
	}
	want := []string{"@@ -1,31 +1,31 @@", "@@ -32,9 +32,9 @@"}
	if strings.Join(heads, "|") != strings.Join(want, "|") {
		t.Fatalf("hunks = %q, want %q", heads, want)
	}
	if n != 40 {
		t.Fatalf("new-side lines = %d, want all 40", n)
	}
}

func TestUnchangedRunsFoldAndOpen(t *testing.T) {
	_, _, p := foldRepo(t)
	// Lines 9..31: between the changes, less the 3 kept beside each. Line 1
	// above the first change and lines 39-40 below the last are too few to fold.
	got := foldRows(p)
	if len(got) != 1 || !strings.HasPrefix(got[0], "⋯  23 unchanged lines") {
		t.Fatalf("fold rows = %q", got)
	}

	// ⏎ on a fold row in the diff opens it.
	p.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	for i := range p.detailLines() {
		if p.foldAt(i) >= 0 && strings.Contains(plainRows(p)[i], "23 unchanged") {
			p.setDiffCursor(i)
			break
		}
	}
	p.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if got := foldRows(p); len(got) != 0 {
		t.Fatalf("after ⏎ = %q", got)
	}
	if !strings.Contains(strings.Join(plainRows(p), "\n"), " line 20") {
		t.Fatal("the opened lines are not shown")
	}

	// F shows the whole file, F again folds everything back.
	p.HandleKey(key("F"))
	if got := foldRows(p); len(got) != 0 || !p.FullFile {
		t.Fatalf("whole file still folds: %q", got)
	}
	p.HandleKey(key("F"))
	if got := foldRows(p); len(got) != 1 {
		t.Fatalf("folded again = %q", got)
	}
}

func TestFoldRowsInTheSplitViewKeepLineNumbers(t *testing.T) {
	_, _, p := foldRepo(t)
	p.Split = true
	rows := plainRows(p)
	joined := strings.Join(rows, "\n")
	if !strings.Contains(joined, "23 unchanged lines") {
		t.Fatalf("no fold row in split view:\n%s", joined)
	}
	// The row after the long fold is line 32, on both sides.
	for i, r := range rows {
		if strings.Contains(r, "23 unchanged lines") {
			for _, next := range rows[i+1:] {
				if strings.Contains(next, "line 32") {
					if !strings.HasPrefix(strings.TrimSpace(next), "32 ") {
						t.Fatalf("row after the fold = %q", next)
					}
					return
				}
			}
		}
	}
	t.Fatalf("line 32 not found:\n%s", joined)
}

// A click on a fold row opens it; one on a hunk's [ + stage ] stages that
// change alone even though the diff carries the whole file.
func TestClickFoldThenStageOneChange(t *testing.T) {
	r, _, p := foldRepo(t)
	for i := range p.detailLines() {
		if p.foldAt(i) >= 0 {
			p.HandleMouse(tea.MouseMsg{X: p.rightX() + 2, Y: p.detailTop() + i - p.detailScroll, Type: tea.MouseLeft, Action: tea.MouseActionPress})
			break
		}
	}
	if got := foldRows(p); len(got) != 0 {
		t.Fatalf("after the click = %q", got)
	}
	p.FocusDiff = true
	for i, s := range p.detailSrc {
		if p.detail[s[0]] == "+THIRTY-FIVE" {
			p.setDiffCursor(i)
			break
		}
	}
	p.HandleKey(key("s"))
	staged := r.Diff(gitx.FileStatus{Path: "long.txt"}, true, false)
	if !strings.Contains(staged, "+THIRTY-FIVE") || strings.Contains(staged, "+FIVE") {
		t.Fatalf("index after staging the second change:\n%s", staged)
	}
}
