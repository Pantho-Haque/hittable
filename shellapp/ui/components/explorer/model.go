// Package explorer defines the explorer component model and types.
package explorer

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/hittable/shellapp/internal/explorer"
)

type ExplorerComponent struct {
	Root     *explorer.FileNode
	Visible  []*explorer.FileNode
	Cursor   int
	HoverRow int
	Focused  bool
	Width    int
	Height   int

	OnFileOpen   func(path string)
	OnFolderOpen func(path string, expanded bool)

	ContextMenuOpen  bool
	ContextMenuIdx   int
	ContextMenuItems []string
	ContextMenuRow   int

	Renaming      bool
	RenameInput   textinput.Model
	RenameNodeIdx int

	AddingFile    bool
	AddingFolder  bool
	AddInput      textinput.Model
	AddTargetPath string

	Deleting      bool
	DeleteNodeIdx int

	lastFocused bool
}

type RenameConfirmMsg struct{}
type DeleteConfirmMsg struct{}
type AddConfirmMsg struct{}

func New(root *explorer.FileNode) *ExplorerComponent {
	vis := explorer.VisibleNodes(root)
	ti := textinput.New()
	ti.CharLimit = 100
	ti.Width = 30
	ti2 := textinput.New()
	ti2.CharLimit = 100
	ti2.Width = 30
	return &ExplorerComponent{
		Root:             root,
		Visible:          vis,
		Cursor:           0,
		HoverRow:         -1,
		ContextMenuItems: []string{"New File", "New Folder", "Rename", "Delete"},
		RenameInput:      ti,
		AddInput:         ti2,
	}
}

func (e *ExplorerComponent) SetSize(w, h int) {
	e.Width = w
	e.Height = h
}

func (e *ExplorerComponent) refreshVisible() {
	e.Visible = explorer.VisibleNodes(e.Root)
	if len(e.Visible) == 0 {
		e.Cursor = 0
		return
	}
	if e.Cursor >= len(e.Visible) {
		e.Cursor = len(e.Visible) - 1
	}
	if e.Cursor < 0 {
		e.Cursor = 0
	}
}

func (e *ExplorerComponent) RebuildTree() {
	expandedPaths := collectExpanded(e.Root)
	newRoot, err := explorer.BuildTree(e.Root.Path)
	if err == nil {
		applyExpanded(newRoot, expandedPaths)
		e.Root = newRoot
	}
	e.refreshVisible()
}

func collectExpanded(node *explorer.FileNode) map[string]bool {
	expanded := make(map[string]bool)
	var walk func(n *explorer.FileNode)
	walk = func(n *explorer.FileNode) {
		if n.Kind == explorer.KindDir && n.Expanded {
			expanded[n.Path] = true
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(node)
	return expanded
}

func applyExpanded(node *explorer.FileNode, expanded map[string]bool) {
	if expanded[node.Path] {
		node.Expanded = true
	}
	for _, child := range node.Children {
		applyExpanded(child, expanded)
	}
}
