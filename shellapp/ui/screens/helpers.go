package screens

import (
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/internal/explorer"
	"github.com/hittable/shellapp/internal/hitfile"
)

var teaProgram *tea.Program

func SetTeaProgram(p *tea.Program) {
	teaProgram = p
}

type responseMsg struct {
	result *hitfile.Response
	err    error
}

type debounceSaveMsg struct{}

var saveDebounceTimer *time.Timer

func (m *MainScreen) saveAndEnqueueDebounced() {
	if saveDebounceTimer != nil {
		saveDebounceTimer.Stop()
	}
	saveDebounceTimer = time.AfterFunc(300*time.Millisecond, func() {
		if teaProgram != nil {
			teaProgram.Send(debounceSaveMsg{})
		}
	})
}

func (m *MainScreen) relPath() string {
	if m.ActiveFile == "" {
		return ""
	}
	rel, err := filepath.Rel(rootDirGlobal, m.ActiveFile)
	if err != nil {
		return filepath.Base(m.ActiveFile)
	}
	return rel
}

func (m *MainScreen) isVimteaInsertOrVisual() bool {
	if m.Focus != FocusTextEditor {
		return false
	}
	return m.TextEd.InInsert || m.TextEd.InVisual
}

func getNodeTargetDir(m *MainScreen) string {
	if m.Explorer.Cursor >= 0 && m.Explorer.Cursor < len(m.Explorer.Visible) {
		node := m.Explorer.Visible[m.Explorer.Cursor]
		if node.Kind == explorer.KindDir {
			return node.Path
		} else if node.Parent != nil {
			return node.Parent.Path
		}
	}
	return ""
}

func interpolateString(s string, env map[string]string) string {
	result := ""
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '<' && s[i+1] == '<' {
			end := i + 2
			for end < len(s) && !(s[end] == '>' && end+1 < len(s) && s[end+1] == '>') {
				end++
			}
			if end < len(s) {
				key := s[i+2 : end]
				if val, ok := env[key]; ok {
					result += val
				} else {
					result += s[i : end+2]
				}
				i = end + 2
				continue
			}
		}
		result += string(s[i])
		i++
	}
	return result
}
