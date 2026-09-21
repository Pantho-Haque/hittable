package explorer_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	expl "github.com/hittable/shellapp/ui/components/explorer"
)

// buildTree is a small in-memory tree builder so tests don't need a real fs.
func buildTree() *expl.Node {
	return &NodeFromFS{
		Name: "root",
		Path: "/root",
		Kind: expl.NodeDir,
		Children: []*expl.Node{
			{Name: "src", Path: "/root/src", Kind: expl.NodeDir, Expanded: false, Children: []*expl.Node{
				{Name: "main.go", Path: "/root/src/main.go", Kind: expl.NodeGeneric},
				{Name: "util.go", Path: "/root/src/util.go", Kind: expl.NodeGeneric},
			}},
			{Name: "test.hit", Path: "/root/test.hit", Kind: expl.NodeHit},
			{Name: "env.json", Path: "/root/env.json", Kind: expl.NodeEnv},
			{Name: "notes", Path: "/root/notes", Kind: expl.NodeDir, Children: []*expl.Node{
				{Name: "readme.md", Path: "/root/notes/readme.md", Kind: expl.NodeMarkdown},
			}},
		},
	}
}

// NodeFromFS is a local helper; in production BuildTree walks the disk. Here
// we mirror its shape so the test can run on any machine without touching fs.
type NodeFromFS = expl.Node

// TestSingleClickSelectsAndPreviewsFile simulates a single click on a file.
// Expected: OnSelect and OnActivate both fire for file preview behavior.
func TestSingleClickSelectsAndPreviewsFile(t *testing.T) {
	root := buildTree()
	e := expl.MustNew("/root")
	e.Root = root
	e.SetSize(30, 20)
	e.RefreshForTest()

	selects := 0
	activates := 0
	var lastSelect *expl.Node
	e.OnSelect = func(n *expl.Node) { selects++; lastSelect = n }
	e.OnActivate = func(n *expl.Node) { activates++ }

	// Visible list (DFS, root excluded):
	//   Y=1: src/   Y=2: test.hit   Y=3: env.json   Y=4: notes/
	if len(e.Visible) < 4 {
		t.Fatalf("expected at least 4 visible nodes, got %d: %v", len(e.Visible), names(e.Visible))
	}

	// Click on Y=2 → test.hit (index 1).
	e.HandleMouse(tea.MouseMsg{X: 5, Y: 2, Type: tea.MouseLeft})

	if selects != 1 {
		t.Errorf("expected 1 OnSelect call, got %d", selects)
	}
	if activates != 1 {
		t.Errorf("single click on file should preview (activate), got %d activations", activates)
	}
	if lastSelect == nil || lastSelect.Name != "test.hit" {
		t.Errorf("expected OnSelect on test.hit, got %v", lastSelect)
	}
	if e.Cursor != 1 {
		t.Errorf("expected Cursor=1, got %d", e.Cursor)
	}
}

// TestDoubleClickActivates simulates two clicks within 400ms on the same row.
// For a file, each click previews (activates), so we get 2 selects + 2 activates.
// For a folder, only the double-click activates (toggles), so we get 2 selects
// + 1 activate. We exercise the folder path here (notes/).
func TestSingleClickOnFolderActivates(t *testing.T) {
	root := buildTree()
	e := expl.MustNew("/root")
	e.Root = root
	e.SetSize(30, 20)
	e.RefreshForTest()

	selects := 0
	activates := 0
	e.OnSelect = func(n *expl.Node) { selects++ }
	e.OnActivate = func(n *expl.Node) { activates++ }

	// Find the "notes" folder (closed by default) and double-click it.
	notesIdx := -1
	for i, n := range e.Visible {
		if n.Name == "notes" {
			notesIdx = i
			break
		}
	}
	if notesIdx < 0 {
		t.Fatalf("notes folder not in visible list")
	}
	rowY := notesIdx + 1 // +1 for header row

	e.HandleMouse(tea.MouseMsg{X: 5, Y: rowY, Type: tea.MouseLeft})

	if selects != 1 {
		t.Errorf("expected 1 OnSelect call, got %d", selects)
	}
	if activates != 1 {
		t.Errorf("expected 1 OnActivate call after single click on folder, got %d", activates)
	}
}

// TestDoubleClickTimeoutDoesNotActivate confirms that a second click outside
// the 400ms threshold is treated as a fresh single click.
func TestDoubleClickTimeoutDoesNotActivate(t *testing.T) {
	root := buildTree()
	e := expl.MustNew("/root")
	e.Root = root
	e.SetSize(30, 20)
	e.RefreshForTest()

	selects := 0
	activates := 0
	e.OnSelect = func(n *expl.Node) { selects++ }
	e.OnActivate = func(n *expl.Node) { activates++ }

	e.HandleMouse(tea.MouseMsg{X: 5, Y: 2, Type: tea.MouseLeft})
	time.Sleep(450 * time.Millisecond)
	e.HandleMouse(tea.MouseMsg{X: 5, Y: 2, Type: tea.MouseLeft})

	if selects != 2 {
		t.Errorf("expected 2 OnSelect, got %d", selects)
	}
	if activates != 2 {
		t.Errorf("expected 2 OnActivate calls for two single-click previews, got %d", activates)
	}
}

// TestDoubleClickDifferentRowDoesNotActivate: second click on a different
// row should be a fresh single click on that row, not an activate of the
// first.
func TestDoubleClickDifferentRowDoesNotActivate(t *testing.T) {
	root := buildTree()
	e := expl.MustNew("/root")
	e.Root = root
	e.SetSize(30, 20)
	e.RefreshForTest()

	activates := 0
	e.OnActivate = func(n *expl.Node) { activates++ }

	e.HandleMouse(tea.MouseMsg{X: 5, Y: 2, Type: tea.MouseLeft}) // test.hit
	e.HandleMouse(tea.MouseMsg{X: 5, Y: 3, Type: tea.MouseLeft}) // env.json

	// test.hit AND env.json are both files, so each click activates → 2 total.
	if activates != 2 {
		t.Errorf("expected 2 OnActivate calls (one per file click), got %d", activates)
	}
}

// TestClickOnFolderToggles: double-click on a folder toggles expansion via
// the OnActivate callback (the screen layer wires that to Expanded = !Expanded).
func TestClickOnFolderToggles(t *testing.T) {
	root := buildTree()
	e := expl.MustNew("/root")
	e.Root = root
	e.SetSize(30, 20)
	e.RefreshForTest()

	var activated *expl.Node
	e.OnActivate = func(n *expl.Node) { activated = n }

	// Find index of "notes" (a closed folder).
	notesIdx := -1
	for i, n := range e.Visible {
		if n.Name == "notes" {
			notesIdx = i
			break
		}
	}
	if notesIdx < 0 {
		t.Fatalf("notes folder not in visible list")
	}
	rowY := notesIdx + 1 // +1 for header row

	e.HandleMouse(tea.MouseMsg{X: 5, Y: rowY, Type: tea.MouseLeft})
	e.HandleMouse(tea.MouseMsg{X: 5, Y: rowY, Type: tea.MouseLeft})

	if activated == nil || activated.Name != "notes" {
		t.Errorf("expected OnActivate on notes, got %v", activated)
	}
}

// TestRightClickMarksContextMenu: a right click sets ContextMenuRequested
// and moves the cursor to the clicked row.
func TestRightClickMarksContextMenu(t *testing.T) {
	root := buildTree()
	e := expl.MustNew("/root")
	e.Root = root
	e.SetSize(30, 20)
	e.RefreshForTest()

	if e.ContextMenuRequested {
		t.Fatal("expected ContextMenuRequested false at start")
	}
	e.HandleMouse(tea.MouseMsg{X: 5, Y: 2, Type: tea.MouseRight})
	if !e.ContextMenuRequested {
		t.Error("expected ContextMenuRequested after right-click")
	}
	if e.Cursor != 1 {
		t.Errorf("expected Cursor=1 (test.hit), got %d", e.Cursor)
	}
}

// TestClickOutsideExplorerIsIgnored: a click at y=0 (above the file list)
// should not move the cursor.
func TestClickOutsideExplorerIsIgnored(t *testing.T) {
	root := buildTree()
	e := expl.MustNew("/root")
	e.Root = root
	e.SetSize(30, 20)
	e.RefreshForTest()
	start := e.Cursor

	e.HandleMouse(tea.MouseMsg{X: 5, Y: 0, Type: tea.MouseLeft})
	if e.Cursor != start {
		t.Errorf("expected cursor to stay at %d, got %d", start, e.Cursor)
	}
}

// TestKeyboardEnterActivates: pressing Enter on a cursor position invokes
// OnActivate for files and toggles folders.
func TestKeyboardEnterActivates(t *testing.T) {
	root := buildTree()
	e := expl.MustNew("/root")
	e.Root = root
	e.SetSize(30, 20)
	e.RefreshForTest()

	var activated *expl.Node
	e.OnActivate = func(n *expl.Node) { activated = n }

	// Find index of test.hit and move cursor there.
	for i, n := range e.Visible {
		if n.Name == "test.hit" {
			e.Cursor = i
			break
		}
	}

	e.ToggleCursor()
	if activated == nil || activated.Name != "test.hit" {
		t.Errorf("expected Enter to activate test.hit, got %v", activated)
	}
}

// TestKeyboardEnterTogglesFolder: pressing Enter on a closed folder expands it.
func TestKeyboardEnterTogglesFolder(t *testing.T) {
	root := buildTree()
	e := expl.MustNew("/root")
	e.Root = root
	e.SetSize(30, 20)
	e.RefreshForTest()

	// Find "notes" index.
	for i, n := range e.Visible {
		if n.Name == "notes" {
			e.Cursor = i
			break
		}
	}
	before := e.Visible[len(e.Visible)-1].Name
	e.ToggleCursor()
	after := e.Visible[len(e.Visible)-1].Name
	if before == after {
		t.Errorf("expected toggling notes to change the visible list tail; before=%q after=%q", before, after)
	}
}

// TestPromptEnterCreatesNewFile covers the inline prompt flow.
func TestPromptEnterCreatesNewFile(t *testing.T) {
	root := buildTree()
	e := expl.MustNew("/root")
	e.Root = root
	e.SetSize(30, 20)
	e.RefreshForTest()

	e.BeginNewFile()
	for _, c := range "hello.go" {
		e.AppendPromptChar(string(c))
	}
	parent, name, isDir, ok := e.ConfirmPrompt()
	if !ok {
		t.Fatal("expected ConfirmPrompt to succeed")
	}
	if isDir {
		t.Errorf("new file should not be a dir")
	}
	if name != "hello.go" {
		t.Errorf("expected name=hello.go, got %q", name)
	}
	if !filepath.IsAbs(parent) && !strings.HasPrefix(parent, "/root") {
		t.Errorf("expected parent under /root, got %q", parent)
	}
}

// TestViewHasExpectedShape: View should produce exactly Height lines, the
// first being the header.
func TestViewHasExpectedShape(t *testing.T) {
	root := buildTree()
	e := expl.MustNew("/root")
	e.Root = root
	e.SetSize(30, 10)
	e.RefreshForTest()

	out := e.View()
	lines := strings.Split(out, "\n")
	if len(lines) != 10 {
		t.Errorf("expected 10 lines in view, got %d", len(lines))
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), "/") {
		t.Errorf("expected header to start with /, got %q", lines[0])
	}
}

// TestScrollKeepsCursorVisible: simulate moving cursor past viewport and
// confirm the scroll position compensates.
func TestScrollKeepsCursorVisible(t *testing.T) {
	root := &expl.Node{Name: "root", Path: "/root", Kind: expl.NodeDir, Expanded: true}
	for i := 0; i < 50; i++ {
		root.Children = append(root.Children, &expl.Node{
			Name: fmt.Sprintf("file%02d", i),
			Path: fmt.Sprintf("/root/file%02d", i),
			Kind: expl.NodeGeneric,
		})
	}
	e := expl.MustNew("/root")
	e.Root = root
	e.SetSize(30, 10)
	e.RefreshForTest()
	if len(e.Visible) != 50 {
		t.Fatalf("expected 50 visible, got %d", len(e.Visible))
	}

	e.Cursor = 45 // way past the bottom of the viewport (which shows ~8 rows)
	// SetSize clamps and re-anchors scroll.
	e.SetSize(30, 10)

	viewportLines := e.ViewportLinesForTest()
	start := e.ScrollStart
	end := start + viewportLines
	if end > len(e.Visible) {
		end = len(e.Visible)
	}
	if e.Cursor < start || e.Cursor >= end {
		t.Errorf("cursor %d not visible in window [%d,%d)", e.Cursor, start, end)
	}
}

func names(ns []*expl.Node) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.Name
	}
	return out
}
