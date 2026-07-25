package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/hitfile"
	"github.com/hittable/shellapp/ui/components/requesteditor"
)

func (m *MainScreen) handleEsc() (tea.Model, tea.Cmd) {
	if m.ExplorerFocused {
		return m, nil
	}
	if m.Focus == FocusResponse && m.Response.SearchOpen {
		m.Response.CloseSearch()
		return m, nil
	}
	if m.URLBar.DropdownOpen {
		m.URLBar.CloseDropdown()
		m.Focus = FocusURLBar
		return m, nil
	}
	if m.Explorer.ContextMenuOpen {
		m.Explorer.CloseContextMenu()
		return m, nil
	}
	if m.Explorer.Renaming {
		m.Explorer.Renaming = false
		return m, nil
	}
	if m.Explorer.AddingFile || m.Explorer.AddingFolder {
		m.Explorer.AddingFile = false
		m.Explorer.AddingFolder = false
		return m, nil
	}
	if m.Explorer.Deleting {
		m.Explorer.Deleting = false
		return m, nil
	}
	if m.Focus == FocusTextEditor && m.ActiveFile != "" {
		if m.isVimteaInsertOrVisual() {
			return m, nil
		}
		m.closeFile()
		return m, nil
	}
	if m.ActiveFile != "" {
		m.closeFile()
		return m, nil
	}
	return m, nil
}

func (m *MainScreen) handleTab() (tea.Model, tea.Cmd) {
	if m.ActiveFile == "" {
		return m, nil
	}
	doc := m.Store.Get(m.ActiveFile)
	if doc == nil {
		return m, nil
	}
	if doc.Kind == document.KindHit && m.ViewMode == ViewRunner {
		m.URLBar.Blur()
		m.Params.Blur()
		m.Headers.Blur()
		m.Body.Blur()
		switch m.Focus {
		case FocusURLBar:
			m.Focus = FocusTabBar
		case FocusTabBar:
			m.Focus = FocusBody
		case FocusBody:
			m.Focus = FocusResponse
		case FocusResponse:
			m.Focus = FocusURLBar
		default:
			m.Focus = FocusURLBar
		}
		switch m.Focus {
		case FocusURLBar:
			m.URLBar.Focus()
		case FocusBody:
			switch m.ActiveTab {
			case requesteditor.TabParams:
				m.Params.Focus()
			case requesteditor.TabHeaders:
				m.Headers.Focus()
			case requesteditor.TabBody:
				m.Body.Focus()
			}
		}
	} else {
		m.TextEd.Blur()
		if m.Focus == FocusTextEditor {
			m.Focus = FocusExplorerPane
			m.ExplorerFocused = true
			m.Explorer.EnsureCursorValid()
		} else {
			m.Focus = FocusTextEditor
			m.TextEd.Focus()
		}
	}
	return m, nil
}

func (m *MainScreen) handleShiftTab() (tea.Model, tea.Cmd) {
	if m.ActiveFile == "" {
		return m, nil
	}
	doc := m.Store.Get(m.ActiveFile)
	if doc == nil {
		return m, nil
	}
	if doc.Kind == document.KindHit && m.ViewMode == ViewRunner {
		m.URLBar.Blur()
		m.Params.Blur()
		m.Headers.Blur()
		m.Body.Blur()
		switch m.Focus {
		case FocusURLBar:
			m.Focus = FocusResponse
		case FocusTabBar:
			m.Focus = FocusURLBar
		case FocusBody:
			m.Focus = FocusTabBar
		case FocusResponse:
			m.Focus = FocusBody
		default:
			m.Focus = FocusResponse
		}
		switch m.Focus {
		case FocusURLBar:
			m.URLBar.Focus()
		case FocusBody:
			switch m.ActiveTab {
			case requesteditor.TabParams:
				m.Params.Focus()
			case requesteditor.TabHeaders:
				m.Headers.Focus()
			case requesteditor.TabBody:
				m.Body.Focus()
			}
		}
	} else {
		m.TextEd.Blur()
		if m.Focus == FocusTextEditor {
			m.Focus = FocusExplorerPane
			m.ExplorerFocused = true
			m.Explorer.EnsureCursorValid()
		} else {
			m.Focus = FocusTextEditor
			m.TextEd.Focus()
		}
	}
	return m, nil
}

func (m *MainScreen) handleCopyCurl() tea.Cmd {
	if m.ActiveFile == "" {
		return nil
	}
	doc := m.Store.Get(m.ActiveFile)
	if doc == nil || doc.Kind != document.KindHit {
		return nil
	}
	method, _ := m.URLBar.GetContent()
	url := m.URLBar.URLInput.Value()
	hdrs := m.Headers.GetContent()
	params := m.Params.GetContent()
	body := m.Body.GetContent()
	h := &hitfile.HitFile{
		Method: method, URL: url, Headers: hdrs, Params: params, Body: body,
	}
	resolved := hitfile.Resolve(h, m.EnvData)
	curl := resolved.AsCurl()
	m.StatusBar = curl
	return nil
}
