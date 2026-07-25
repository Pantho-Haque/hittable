// Package explorer implements context menu, rename, add, and delete state management.
package explorer

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/internal/explorer"
)

func (e *ExplorerComponent) CloseContextMenu() {
	e.ContextMenuOpen = false
	e.ContextMenuIdx = 0
}

func (e *ExplorerComponent) OpenContextMenu() {
	if e.Cursor < 0 || e.Cursor >= len(e.Visible) {
		return
	}
	e.ContextMenuOpen = true
	e.ContextMenuIdx = 0
	e.ContextMenuRow = e.Cursor
}

func (e *ExplorerComponent) OpenContextMenuAtRow(row int) {
	if row < 0 || row >= len(e.Visible) {
		return
	}
	e.Cursor = row
	e.ContextMenuOpen = true
	e.ContextMenuIdx = 0
	e.ContextMenuRow = row
}

func (e *ExplorerComponent) StartRename() {
	if e.Cursor < 0 || e.Cursor >= len(e.Visible) {
		return
	}
	node := e.Visible[e.Cursor]
	e.Renaming = true
	e.RenameNodeIdx = e.Cursor
	e.RenameInput.SetValue(node.Name)
	e.RenameInput.Focus()
}

func (e *ExplorerComponent) StartAddFile(targetDir string) {
	e.AddingFile = true
	e.AddTargetPath = targetDir
	e.AddInput.SetValue("")
	e.AddInput.Focus()
}

func (e *ExplorerComponent) StartAddFolder(targetDir string) {
	e.AddingFolder = true
	e.AddTargetPath = targetDir
	e.AddInput.SetValue("")
	e.AddInput.Focus()
}

func (e *ExplorerComponent) StartDelete() {
	if e.Cursor < 0 || e.Cursor >= len(e.Visible) {
		return
	}
	e.Deleting = true
	e.DeleteNodeIdx = e.Cursor
}

func (e *ExplorerComponent) ConfirmDelete() (string, bool) {
	if e.DeleteNodeIdx < 0 || e.DeleteNodeIdx >= len(e.Visible) {
		e.Deleting = false
		return "", false
	}
	node := e.Visible[e.DeleteNodeIdx]
	path := node.Path
	e.Deleting = false
	e.DeleteNodeIdx = -1
	return path, true
}

func (e *ExplorerComponent) ConfirmRename() (string, string, bool) {
	if e.RenameNodeIdx < 0 || e.RenameNodeIdx >= len(e.Visible) {
		e.Renaming = false
		return "", "", false
	}
	node := e.Visible[e.RenameNodeIdx]
	oldPath := node.Path
	newName := strings.TrimSpace(e.RenameInput.Value())
	if newName == "" || newName == node.Name {
		e.Renaming = false
		return "", "", false
	}
	newPath := filepath.Join(filepath.Dir(oldPath), newName)
	e.Renaming = false
	return oldPath, newPath, true
}

func (e *ExplorerComponent) ConfirmAdd() (string, string, bool) {
	name := strings.TrimSpace(e.AddInput.Value())
	if name == "" {
		e.AddingFile = false
		e.AddingFolder = false
		return "", "", false
	}
	basePath := e.AddTargetPath
	if basePath == "" {
		basePath = e.Root.Path
	}
	newPath := filepath.Join(basePath, name)
	isDir := e.AddingFolder
	e.AddingFile = false
	e.AddingFolder = false
	return newPath, name, isDir
}

func (e *ExplorerComponent) handleContextMenuInput(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if e.ContextMenuIdx > 0 {
			e.ContextMenuIdx--
		}
	case "down", "j":
		if e.ContextMenuIdx < len(e.ContextMenuItems)-1 {
			e.ContextMenuIdx++
		}
	case "enter", " ":
		idx := e.ContextMenuIdx
		e.CloseContextMenu()
		target := ""
		if e.Cursor >= 0 && e.Cursor < len(e.Visible) {
			node := e.Visible[e.Cursor]
			if node.Kind == explorer.KindDir {
				target = node.Path
			} else if node.Parent != nil {
				target = node.Parent.Path
			}
		}
		switch idx {
		case 0:
			e.StartAddFile(target)
		case 1:
			e.StartAddFolder(target)
		case 2:
			e.StartRename()
		case 3:
			e.StartDelete()
		}
		return nil
	case "esc", "x":
		e.CloseContextMenu()
		return nil
	}
	return nil
}

func (e *ExplorerComponent) getContextMenuTargetDir() string {
	if e.Cursor >= 0 && e.Cursor < len(e.Visible) {
		node := e.Visible[e.Cursor]
		if node.Kind == explorer.KindDir {
			return node.Path
		} else if node.Parent != nil {
			return node.Parent.Path
		}
	}
	return ""
}
