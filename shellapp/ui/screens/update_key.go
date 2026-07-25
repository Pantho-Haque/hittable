package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/ui/components/requesteditor"
)

func (m *MainScreen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if msg.String() == "ctrl+b" {
		return m, m.toggleExplorerFocus()
	}
	if m.ExplorerFocused {
		return m, m.handleExplorerKey(msg)
	}
	if m.Focus == FocusMethodDropdown {
		return m, m.handleDropdownKey(msg)
	}
	if m.Focus == FocusResponse && m.Response.SearchOpen {
		return m, m.handleResponseSearchKey(msg)
	}
	if msg.String() == "ctrl+f" && m.ActiveFile != "" {
		m.Response.OpenSearch()
		m.Focus = FocusResponse
		return m, nil
	}
	if msg.String() == "ctrl+t" && m.ActiveFile != "" {
		m.toggleViewMode()
		return m, nil
	}
	if (msg.String() == "ctrl+enter" || msg.String() == "ctrl+r") && m.ActiveFile != "" {
		doc := m.Store.Get(m.ActiveFile)
		if doc != nil && doc.Kind == document.KindHit {
			m.saveAndEnqueue()
			m.sendRequestAsync()
			return m, nil
		}
	}
	if msg.String() == "ctrl+s" {
		m.saveAndEnqueue()
		m.StatusBar = "Saved"
		return m, nil
	}
	if msg.String() == "esc" {
		return m.handleEsc()
	}
	if msg.String() == "tab" {
		return m.handleTab()
	}
	if msg.String() == "shift+tab" {
		return m.handleShiftTab()
	}
	if msg.String() == "ctrl+shift+c" {
		return m, m.handleCopyCurl()
	}
	if msg.String() == "?" {
		return m, nil
	}
	if msg.String() == "n" && m.Focus == FocusResponse && m.Response.SearchOpen {
		m.Response.NextMatch()
		return m, nil
	}
	if msg.String() == "enter" && m.Focus == FocusURLBar && !m.URLBar.DropdownOpen {
		m.URLBar.ToggleDropdown()
		if m.URLBar.DropdownOpen {
			m.Focus = FocusMethodDropdown
		}
		return m, nil
	}
	if m.Focus == FocusTabBar {
		m.handleTabBarKey(msg)
		return m, nil
	}
	if m.Focus == FocusURLBar {
		cmd := m.URLBar.Update(msg)
		if cmd != nil {
			return m, cmd
		}
	}
	if m.Focus == FocusBody {
		return m, m.handleBodyKey(msg)
	}
	if m.Focus == FocusTextEditor {
		return m, m.handleTextEditorKey(msg)
	}
	if m.Focus == FocusResponse {
		cmd := m.Response.Update(msg)
		if cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

func (m *MainScreen) toggleExplorerFocus() tea.Cmd {
	if m.ExplorerFocused {
		m.ExplorerFocused = false
		m.Focus = m.LastFocus
		switch m.Focus {
		case FocusURLBar:
			m.URLBar.Focus()
		case FocusBody:
			m.Body.Focus()
		case FocusTextEditor:
			m.TextEd.Focus()
		}
	} else {
		m.ExplorerFocused = true
		m.Explorer.EnsureCursorValid()
		m.LastFocus = m.Focus
		m.URLBar.Blur()
		m.Params.Blur()
		m.Headers.Blur()
		m.Body.Blur()
		m.TextEd.Blur()
		m.Focus = FocusExplorerPane
	}
	return nil
}

func (m *MainScreen) handleExplorerKey(msg tea.KeyMsg) tea.Cmd {
	if msg.String() == "x" {
		if m.Explorer.ContextMenuOpen {
			m.Explorer.CloseContextMenu()
			return nil
		}
		m.Explorer.OpenContextMenu()
		return nil
	}
	if msg.String() == "ctrl+n" {
		m.Explorer.StartAddFile(getNodeTargetDir(m))
		return nil
	}
	if msg.String() == "ctrl+shift+n" {
		m.Explorer.StartAddFolder(getNodeTargetDir(m))
		return nil
	}
	if msg.String() == "ctrl+e" {
		m.Explorer.StartRename()
		return nil
	}
	if msg.String() == "ctrl+d" {
		m.Explorer.StartDelete()
		return nil
	}
	cmd := m.Explorer.Update(msg)
	if cmd != nil {
		return cmd
	}
	return nil
}

func (m *MainScreen) handleDropdownKey(msg tea.KeyMsg) tea.Cmd {
	if m.URLBar.HandleDropdownKey(msg) {
		if !m.URLBar.DropdownOpen {
			m.Focus = FocusURLBar
			m.URLBar.Focus()
		}
	}
	return nil
}

func (m *MainScreen) handleResponseSearchKey(msg tea.KeyMsg) tea.Cmd {
	if msg.String() == "esc" {
		m.Response.CloseSearch()
		return nil
	}
	cmd := m.Response.Update(msg)
	if cmd != nil {
		return cmd
	}
	return nil
}

func (m *MainScreen) handleTabBarKey(msg tea.KeyMsg) {
	if msg.String() == "left" || msg.String() == "h" {
		if m.ActiveTab > 0 {
			m.ActiveTab--
		}
	} else if msg.String() == "right" || msg.String() == "l" {
		if m.ActiveTab < 2 {
			m.ActiveTab++
		}
	} else if msg.String() == "1" {
		m.ActiveTab = requesteditor.TabParams
	} else if msg.String() == "2" {
		m.ActiveTab = requesteditor.TabHeaders
	} else if msg.String() == "3" {
		m.ActiveTab = requesteditor.TabBody
	}
}

func (m *MainScreen) handleBodyKey(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	switch m.ActiveTab {
	case requesteditor.TabParams:
		cmd = m.Params.Update(msg)
	case requesteditor.TabHeaders:
		cmd = m.Headers.Update(msg)
	case requesteditor.TabBody:
		cmd = m.Body.Update(msg)
	}
	if cmd != nil {
		return cmd
	}
	m.saveAndEnqueueDebounced()
	return nil
}

func (m *MainScreen) handleTextEditorKey(msg tea.KeyMsg) tea.Cmd {
	cmd := m.TextEd.Update(msg)
	if cmd != nil {
		return cmd
	}
	m.saveAndEnqueueDebounced()
	return nil
}
