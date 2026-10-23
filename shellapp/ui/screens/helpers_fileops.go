package screens

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/hittable/shellapp/internal/hitfile"
	"github.com/hittable/shellapp/ui/components/explorer"
)

func (m *MainScreen) applyRename(oldPath, newPath string) {
	if oldPath == newPath {
		return
	}
	if _, err := os.Stat(newPath); err == nil {
		m.StatusBar = "Rename failed: target exists"
		return
	}
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow() // don't let a pending write resurrect oldPath
	if err := os.Rename(oldPath, newPath); err != nil {
		m.StatusBar = "Rename failed"
		return
	}
	// Every open document at or under oldPath moves: a renamed folder takes
	// its open files with it, or their next autosave targets a path that no
	// longer exists and the edit is lost.
	for _, p := range m.Store.Paths() {
		if p != oldPath && !strings.HasPrefix(p, oldPath+string(os.PathSeparator)) {
			continue
		}
		np := newPath + strings.TrimPrefix(p, oldPath)
		if doc := m.Store.Get(p); doc != nil {
			m.Store.Delete(p)
			doc.Path = np
			m.Store.Set(np, doc)
		}
		m.TextEd.RemoveEditor(p)
		if m.ActiveFile == p {
			m.ActiveFile = np
			if m.ViewMode == ViewText {
				m.TextEd.SetContent(np, m.TextEd.GetContent())
			}
		}
	}
	_ = m.Explorer.RebuildTree()
}

func (m *MainScreen) applyDelete(path string) {
	if m.ActiveFile == path || strings.HasPrefix(m.ActiveFile, path+string(os.PathSeparator)) {
		m.ActiveFile = "" // skip the save in closeFile
		m.closeFile()
		m.ViewMode = ViewRunner
	}
	m.WriteQueue.Cancel(path)
	m.Store.Delete(path)
	m.TextEd.RemoveEditor(path)
	if err := os.RemoveAll(path); err != nil {
		m.StatusBar = "Delete failed"
	}
	_ = m.Explorer.RebuildTree()
}

func (m *MainScreen) applyAdd(parent, name string, isDir bool) {
	full := filepath.Join(parent, name)
	if _, err := os.Stat(full); err == nil {
		m.StatusBar = "Already exists: " + name
		return
	}
	if isDir {
		if err := os.MkdirAll(full, 0o755); err != nil {
			m.StatusBar = "Create folder failed"
			return
		}
	} else {
		var content []byte
		if strings.HasSuffix(full, ".hit") {
			content = hitfile.Marshal(&hitfile.HitFile{
				Method:  "GET",
				Headers: map[string]string{"Content-Type": "application/json"},
				Params:  map[string]string{},
			})
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err == nil {
			err = os.WriteFile(full, content, 0o644)
			if err != nil {
				m.StatusBar = "Create file failed"
				return
			}
		}
	}
	_ = m.Explorer.RebuildTree()
	// Reveal it: expand each collapsed folder from the root down to it.
	var dirs []string
	for d := filepath.Dir(full); len(d) > len(m.Explorer.Root.Path); d = filepath.Dir(d) {
		dirs = append([]string{d}, dirs...)
	}
	for _, d := range dirs {
		if n := explorer.FindByPath(m.Explorer.Root, d); n != nil && !n.Expanded {
			m.Explorer.Toggle(n)
		}
	}
	for i, n := range m.Explorer.Visible {
		if n.Path == full {
			m.Explorer.Cursor = i
			break
		}
	}
	if !isDir {
		m.openFileRaw(full)
	}
}
