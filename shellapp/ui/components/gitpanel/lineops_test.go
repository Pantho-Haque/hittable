package gitpanel

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/internal/gitx"
)

// lineRepo commits a ten-line file, then changes its first and last lines so
// the diff has two separate hunks.
func lineRepo(t *testing.T) (*gitx.Repo, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@x")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@x")
	run("config", "user.name", "T")
	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, "line "+strings.Repeat("x", i))
	}
	lines[4] = "\t\tline xxxxx" // tab-indented, like Go
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	run("add", ".")
	run("commit", "-q", "-m", "init")
	lines[0], lines[9] = "FIRST", "LAST"
	os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	return gitx.Open(dir), path
}

// tab moves the keyboard to the diff; s stages the hunk under the cursor, and
// the second hunk stays unstaged. d then reverts the remaining hunk after a
// y confirm, and the working file matches the index.
func TestStageAndRevertHunksFromTheDiff(t *testing.T) {
	repo, path := lineRepo(t)
	p := New(repo)
	p.SetSize(120, 40)
	p.Refresh()
	if r := p.current(); r == nil || r.file == nil || r.file.Path != "f.txt" {
		t.Fatalf("cursor not on f.txt: %+v", r)
	}
	if p.hunkBtns == "" {
		t.Fatal("unstaged tracked file should offer line ops")
	}

	p.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	if !p.FocusDiff {
		t.Fatal("tab should focus the diff")
	}
	// Cursor to the first "+FIRST" row and stage its hunk.
	p.detailLines()
	for i, s := range p.detailSrc {
		if p.detail[s[0]] == "+FIRST" {
			p.setDiffCursor(i)
			break
		}
	}
	p.HandleKey(key("s"))
	staged := repo.Diff(gitx.FileStatus{Path: "f.txt"}, true, false)
	if !strings.Contains(staged, "+FIRST") || strings.Contains(staged, "+LAST") {
		t.Fatalf("index after staging hunk 1:\n%s", staged)
	}

	// The list now has the file in both groups; put the cursor on the
	// unstaged one and revert what is left.
	p.Refresh()
	for i, r := range p.rows {
		if r.file != nil && !r.staged {
			p.Cursor = i
			p.loadDetail()
			break
		}
	}
	if r := p.current(); r == nil || r.staged {
		t.Fatalf("no unstaged row: %+v", r)
	}
	p.FocusDiff = true
	p.setDiffCursor(len(p.detailLines()) - 1) // inside the LAST hunk
	p.HandleKey(key("d"))
	if p.prompt != promptConfirm {
		t.Fatal("revert should ask for confirmation")
	}
	p.HandleKey(key("y"))
	b, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(b), "FIRST\n") || strings.Contains(string(b), "LAST") {
		t.Fatalf("working file after revert:\n%s", b)
	}
	if !strings.Contains(p.Message, "revert lines ✓") {
		t.Errorf("message = %q", p.Message)
	}
}

// Clicking the hunk header's [ + stage ] button stages that hunk alone.
func TestHunkButtonClickStages(t *testing.T) {
	repo, _ := lineRepo(t)
	p := New(repo)
	p.SetSize(120, 40)
	p.Refresh()
	dl := p.detailLines()
	// Find the rendered row of the second hunk header.
	heads := []int{}
	for i, s := range p.detailSrc {
		if strings.HasPrefix(p.detail[s[0]], "@@") && (i == 0 || p.detailSrc[i-1][0] != s[0]) {
			heads = append(heads, i)
		}
	}
	if len(heads) != 2 {
		t.Fatalf("want 2 hunks, got %d in %d rows", len(heads), len(dl))
	}
	y := p.detailTop() + heads[1] - p.detailScroll
	x := p.rightX() + p.detailWidth() - 2 // inside "[ + stage ]"
	if got := p.hunkBtnAt(heads[1], x-p.rightX()); got != "stage" {
		t.Fatalf("button under the mouse = %q", got)
	}
	p.HandleMouse(tea.MouseMsg{Type: tea.MouseLeft, Action: tea.MouseActionPress, X: x, Y: y})
	staged := repo.Diff(gitx.FileStatus{Path: "f.txt"}, true, false)
	if !strings.Contains(staged, "+LAST") || strings.Contains(staged, "+FIRST") {
		t.Fatalf("index after clicking stage on hunk 2:\n%s", staged)
	}
}

// In a split diff the new side is the working copy: a click there opens the
// editor on that line, HEAD stays on the left, and typing lands in the file.
func TestClickNewSideOfSplitEdits(t *testing.T) {
	repo, path := lineRepo(t)
	p := New(repo)
	p.SetSize(120, 40)
	p.Refresh()
	p.Split = true
	dl := p.detailLines()
	last := -1 // rendered row showing the new last line ("LAST")
	for i, l := range dl {
		if strings.Contains(l, "LAST") {
			last = i
		}
	}
	if last < 0 {
		t.Fatal("LAST not in split diff")
	}
	if got := p.newLineAt(last); got != 10 {
		t.Fatalf("newLineAt = %d, want 10", got)
	}
	x := p.dividerX() + 2
	p.HandleMouse(tea.MouseMsg{X: x, Y: listTop + last, Type: tea.MouseLeft, Action: tea.MouseActionPress})
	if !p.Editing || p.Editor.GetCursorRow() != 9 {
		t.Fatalf("editing=%v row=%d", p.Editing, p.Editor.GetCursorRow())
	}
	if len(p.editOld) != 10 || p.editOld[0] != "line x" {
		t.Fatalf("editOld = %q", p.editOld)
	}
	rows := p.rightColumn()
	view := ansi.Strip(strings.Join(rows, "\n"))
	if !strings.Contains(view, "line x") || !strings.Contains(view, "LAST") {
		t.Fatal("split edit view should show HEAD on the left and the editor on the right")
	}
	for i, l := range rows {
		if w := lipgloss.Width(l); w != p.rightW() || strings.Contains(l, "\t") {
			t.Fatalf("row %d is %d cells (want %d) or carries a raw tab: %q", i, w, p.rightW(), ansi.Strip(l))
		}
	}
	if p.sepAt(p.listW()+1, listTop) != "list" || p.ResizeAxis() != "" {
		t.Fatal("sepAt / ResizeAxis")
	}
	// Two hunks (FIRST, LAST): gutter buttons on their rows, red/green tints
	// via the styles (plain in tests), and the buttons act on the hunk.
	if len(p.editHunks) != 2 || !strings.Contains(ansi.Strip(rows[1]), "⟲ + │") || !strings.Contains(ansi.Strip(rows[10]), "⟲ + │") || strings.Contains(ansi.Strip(rows[5]), "⟲") {
		t.Fatalf("hunks %v gutter rows:\n%s", p.editHunks, view)
	}
	half := p.splitHalf()
	p.HandleMouse(tea.MouseMsg{X: p.rightX() + half + 2, Y: listTop + 1, Type: tea.MouseLeft, Action: tea.MouseActionPress}) // + on hunk 1
	if out, _ := repo.Run("diff", "--cached"); !strings.Contains(out, "+FIRST") || strings.Contains(out, "+LAST") {
		t.Fatalf("stage hunk: %q msg=%q", out, p.Message)
	}
	if !p.Editing || len(p.editHunks) != 1 {
		t.Fatalf("editor should stay open with one hunk left: editing=%v hunks=%v", p.Editing, p.editHunks)
	}
	p.HandleMouse(tea.MouseMsg{X: p.rightX() + half, Y: listTop + 10, Type: tea.MouseLeft, Action: tea.MouseActionPress}) // ⟲ on hunk 2
	if c := p.Editor.GetContent(); strings.Contains(c, "LAST") || len(p.editHunks) != 0 {
		t.Fatalf("revert hunk: %q hunks=%v", c, p.editHunks)
	}
	// A pure deletion shows the removed text in red on the row after it.
	p.Editor.SetValue("FIRST\nline xx\nline xxxx\n" + strings.Join(strings.Split(p.Editor.GetContent(), "\n")[4:], "\n"))
	if h := p.hunkAt(2); h == nil || h.ns != h.ne {
		t.Fatalf("deleting line 3 should be a pure deletion at 2: %v", p.editHunks)
	}
	if row := ansi.Strip(p.rightColumn()[3]); !strings.Contains(row, "− line xxx") {
		t.Fatalf("deleted line not shown on the left: %q", row)
	}
	// ⌥z toggles wrap for the panel and its editor together; wheel left/right
	// reach the editor (no-op here, must not panic).
	wrap := p.Wrap
	p.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Ω")})
	p.rightColumn()
	if p.Wrap == wrap || p.Editor.Wrap != p.Wrap {
		t.Fatalf("alt+z: panel wrap %v→%v, editor wrap %v", wrap, p.Wrap, p.Editor.Wrap)
	}
	p.HandleMouse(tea.MouseMsg{X: p.rightX() + half + 10, Y: listTop + 3, Type: tea.MouseWheelRight})
	if p.Wrap {
		p.ToggleWrap()
	}
	p.Editor.SetValue(strings.Repeat("wide ", 40) + "\n" + strings.SplitN(p.Editor.GetContent(), "\n", 2)[1]) // something to pan over
	p.rightColumn()
	p.HandleMouse(tea.MouseMsg{X: p.rightX() + half + 10, Y: listTop + 3, Type: tea.MouseWheelRight}) // 6 cells right
	if row := ansi.Strip(p.rightColumn()[2]); !strings.HasPrefix(row, "  2 x ") {                     // "line xx" panned to "x"
		t.Fatalf("index side should pan with the editor: %q", row)
	}
	p.Editor.GotoLine(8) // the last line, after the deletion above
	p.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	p.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "!line xxxxxxxxxx") || strings.Contains(string(b), "LAST") {
		t.Fatalf("edit not written: %q", b)
	}
}
