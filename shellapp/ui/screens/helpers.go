package screens

import (
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/internal/hitfile"
)

var teaProgram *tea.Program

func SetTeaProgram(p *tea.Program) {
	teaProgram = p
}

type responseMsg struct {
	path   string
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
	rel, err := filepath.Rel(m.RootDir, m.ActiveFile)
	if err != nil {
		return filepath.Base(m.ActiveFile)
	}
	return rel
}
