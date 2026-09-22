// Package screens implements the update logic for the main screen.
package screens

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/ui/components/gitpanel"
	"github.com/hittable/shellapp/ui/components/palette"
	"github.com/hittable/shellapp/ui/components/requesteditor"
	"github.com/hittable/shellapp/ui/components/terminal"
)

type tabClickMsg struct {
	tab requesteditor.Tab
}

type toggleViewMsg struct {
	mode ViewMode
}

type methodSelectMsg struct {
	idx    int
	method string
}

type searchIconClickMsg struct{}

// renameConfirmMsg / deleteConfirmMsg / addConfirmMsg are emitted by the
// explorer when the user confirms an inline prompt.
type renameConfirmMsg struct {
	oldPath string
	newPath string
}
type deleteConfirmMsg struct {
	path string
}
type addConfirmMsg struct {
	parent string
	name   string
	isDir  bool
}

func (m *MainScreen) Init() tea.Cmd {
	return gitTick()
}

func (m *MainScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	model, cmd := m.update(msg)
	// Opening or closing a full-width panel changes the layout. Too many
	// paths flip GitOpen / Palette.Open to re-lay out at each of them, so the
	// change is picked up here instead.
	if hidden := m.explorerHidden(); hidden != m.sidebarWasHidden {
		m.SetSize(m.Width, m.Height)
	}
	return model, cmd
}

func (m *MainScreen) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case responseMsg:
		m.handleResponseMsg(msg)
		return m, nil
	case terminal.OutputMsg:
		return m, nil
	case gitpanel.DoneMsg:
		m.Git.Done(msg)
		return m, nil
	case gitTickMsg:
		return m, m.pollGit()
	case gitStatusMsg:
		m.applyStatus(msg)
		return m, nil
	case palette.ResultsMsg:
		m.Palette.Deliver(msg)
		return m, nil
	case spinner.TickMsg:
		if !m.Sending {
			return m, nil
		}
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	case renameConfirmMsg:
		m.applyRename(msg.oldPath, msg.newPath)
		return m, nil
	case deleteConfirmMsg:
		m.applyDelete(msg.path)
		return m, nil
	case addConfirmMsg:
		m.applyAdd(msg.parent, msg.name, msg.isDir)
		return m, nil
	case tabClickMsg:
		m.ActiveTab = msg.tab
		m.Focus = FocusBody
		m.focusCurrent()
		return m, nil
	case toggleViewMsg:
		if msg.mode == ViewRunner && m.ViewMode != ViewRunner {
			m.toggleViewMode()
		} else if msg.mode == ViewText && m.ViewMode != ViewText {
			m.toggleViewMode()
		}
		return m, nil
	case methodSelectMsg:
		m.URLBar.SelectMethod(msg.idx)
		m.Focus = FocusURLBar
		m.focusCurrent()
		m.saveAndEnqueueDebounced()
		return m, nil
	case searchIconClickMsg:
		m.Response.OpenSearch()
		m.setExplorerFocused(false)
		m.blurAll()
		m.Focus = FocusResponse
		return m, nil
	case debounceSaveMsg:
		m.saveAndEnqueue()
		return m, nil
	}
	return m, nil
}
