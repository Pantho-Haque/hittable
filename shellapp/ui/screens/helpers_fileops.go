package screens

import (
	"encoding/json"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *MainScreen) handleRenameConfirm() (tea.Model, tea.Cmd) {
	oldPath, newPath, ok := m.Explorer.ConfirmRename()
	if !ok {
		return m, nil
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		m.StatusBar = "Rename failed"
		return m, nil
	}
	if m.ActiveFile == oldPath {
		doc := m.Store.Get(oldPath)
		if doc != nil {
			m.Store.Set(newPath, doc)
			m.Store.Delete(oldPath)
			doc.Path = newPath
		}
		m.ActiveFile = newPath
	}
	m.Explorer.RebuildTree()
	return m, nil
}

func (m *MainScreen) handleDeleteConfirm() (tea.Model, tea.Cmd) {
	path, ok := m.Explorer.ConfirmDelete()
	if !ok {
		return m, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		m.StatusBar = "Delete failed"
		return m, nil
	}
	if info.IsDir() {
		os.RemoveAll(path)
	} else {
		os.Remove(path)
	}
	if m.ActiveFile == path {
		m.ActiveFile = ""
		m.ViewMode = ViewRunner
	}
	m.Explorer.RebuildTree()
	return m, nil
}

func (m *MainScreen) handleAddConfirm() (tea.Model, tea.Cmd) {
	newPath, name, isDir := m.Explorer.ConfirmAdd()
	if name == "" {
		return m, nil
	}
	if isDir {
		if err := os.MkdirAll(newPath, 0o755); err != nil {
			m.StatusBar = "Create folder failed"
			return m, nil
		}
	} else {
		var content []byte
		if strings.HasSuffix(newPath, ".hit") {
			hit := map[string]interface{}{
				"method":   "GET",
				"url":      "",
				"headers":  map[string]string{"Content-Type": "application/json"},
				"params":   map[string]string{},
				"body":     "",
				"response": nil,
			}
			data, _ := json.MarshalIndent(hit, "", "  ")
			content = append(data, '\n')
		}
		if err := os.WriteFile(newPath, content, 0o644); err != nil {
			m.StatusBar = "Create file failed"
			return m, nil
		}
	}
	m.Explorer.RebuildTree()
	return m, nil
}

func (m *MainScreen) closeFile() {
	if m.ActiveFile == "" {
		return
	}
	m.ActiveFile = ""
	m.ViewMode = ViewRunner
	m.Focus = FocusExplorerPane
	m.ExplorerFocused = true
	m.Explorer.EnsureCursorValid()
	m.URLBar.Blur()
	m.Params.Blur()
	m.Headers.Blur()
	m.Body.Blur()
	m.TextEd.Blur()
	m.Response.SetResponse(0, "", 0, 0, false, "")
}
