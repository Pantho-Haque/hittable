// Package explorer implements click, hover, and scroll helpers for the explorer.
package explorer

import "github.com/hittable/shellapp/internal/explorer"

func (e *ExplorerComponent) GetScrollStart() int {
	maxLines := e.Height - 2
	if maxLines < 1 {
		maxLines = 1
	}
	start := e.Cursor - (maxLines - 1)
	if start < 0 {
		start = 0
	}
	return start
}

func (e *ExplorerComponent) EnsureCursorValid() {
	e.refreshVisible()
}

func (e *ExplorerComponent) ClickAtRow(absoluteRow int) {
	if absoluteRow < 0 || absoluteRow >= len(e.Visible) {
		return
	}
	e.Cursor = absoluteRow
	node := e.Visible[absoluteRow]
	if node.Kind == explorer.KindDir {
		node.Expanded = !node.Expanded
		e.refreshVisible()
		if e.OnFolderOpen != nil {
			e.OnFolderOpen(node.Path, node.Expanded)
		}
	} else {
		if e.OnFileOpen != nil {
			e.OnFileOpen(node.Path)
		}
	}
}

func (e *ExplorerComponent) HoverAtRow(absoluteRow int) {
	if absoluteRow < 0 || absoluteRow >= len(e.Visible) {
		e.HoverRow = -1
		return
	}
	e.HoverRow = absoluteRow
}
