// Package screens implements the update logic for the main screen.
package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/ui/components/requesteditor"
	explorerui "github.com/hittable/shellapp/ui/components/explorer"
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

type explorerClickMsg struct {
	idx int
}

func (m *MainScreen) Init() tea.Cmd {
	return nil
}

func (m *MainScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case explorerui.RenameConfirmMsg:
		return m.handleRenameConfirm()
	case explorerui.DeleteConfirmMsg:
		return m.handleDeleteConfirm()
	case explorerui.AddConfirmMsg:
		return m.handleAddConfirm()
	case tabClickMsg:
		m.ActiveTab = msg.tab
		m.Focus = FocusBody
		switch msg.tab {
		case requesteditor.TabParams:
			m.Params.Focus()
		case requesteditor.TabHeaders:
			m.Headers.Focus()
		case requesteditor.TabBody:
			m.Body.Focus()
		}
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
		m.URLBar.Focus()
		return m, nil
	case searchIconClickMsg:
		m.Response.OpenSearch()
		m.Focus = FocusResponse
		return m, nil
	case debounceSaveMsg:
		m.saveAndEnqueue()
		return m, nil
	case explorerClickMsg:
		m.handleExplorerClick(msg.idx)
		return m, nil
	}
	return m, nil
}
