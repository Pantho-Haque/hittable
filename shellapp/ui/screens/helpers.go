package screens

import (
	"path/filepath"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/internal/hitfile"
)

// prog is the running program. Goroutines (request, git, debounce timers,
// the terminal reader) read it while SetTeaProgram may write it, so it is
// atomic.
var prog atomic.Pointer[tea.Program]

// send delivers msg to the running program; a no-op before one exists.
func send(msg tea.Msg) bool {
	if p := prog.Load(); p != nil {
		p.Send(msg)
		return true
	}
	return false
}

// InitialStatus is shown in the footer on the first frame (e.g. an import
// summary from the CLI).
var InitialStatus string

func SetTeaProgram(p *tea.Program) {
	prog.Store(p)
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
		send(debounceSaveMsg{})
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
