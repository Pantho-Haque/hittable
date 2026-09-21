package screens

import (
	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/hitfile"
)

func (m *MainScreen) handleEsc() (tea.Model, tea.Cmd) {
	if m.ShowHelp {
		m.ShowHelp = false
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
	if m.ActiveFile != "" {
		m.closeFile()
	}
	return m, nil
}

// runnerOrder is the Tab focus cycle in Runner view.
var runnerOrder = []FocusArea{FocusURLBar, FocusTabBar, FocusBody, FocusResponse}

func (m *MainScreen) cycleFocus(dir int) (tea.Model, tea.Cmd) {
	if m.ActiveFile == "" {
		return m, nil
	}
	doc := m.Store.Get(m.ActiveFile)
	if doc == nil {
		return m, nil
	}
	if doc.Kind == document.KindHit && m.ViewMode == ViewRunner {
		cur := 0
		for i, f := range runnerOrder {
			if f == m.Focus {
				cur = i
			}
		}
		m.Focus = runnerOrder[(cur+dir+len(runnerOrder))%len(runnerOrder)]
		m.focusCurrent()
		return m, nil
	}
	// Text view: Tab toggles editor <-> explorer.
	if m.Focus == FocusTextEditor {
		m.blurAll()
		m.LastFocus = FocusTextEditor
		m.Focus = FocusExplorerPane
		m.setExplorerFocused(true)
	} else {
		m.Focus = FocusTextEditor
		m.focusCurrent()
	}
	return m, nil
}

func (m *MainScreen) handleTab() (tea.Model, tea.Cmd)      { return m.cycleFocus(1) }
func (m *MainScreen) handleShiftTab() (tea.Model, tea.Cmd) { return m.cycleFocus(-1) }

func (m *MainScreen) handleCopyCurl() tea.Cmd {
	h := m.currentHit()
	if h == nil {
		return nil
	}
	curl := hitfile.Resolve(h, m.EnvData).AsCurl()
	if err := clipboard.WriteAll(curl); err != nil {
		m.StatusBar = curl
		return nil
	}
	m.StatusBar = "Copied as curl"
	return nil
}

func (m *MainScreen) copyResponseBody() tea.Cmd {
	if m.Response.Empty() {
		return nil
	}
	if err := clipboard.WriteAll(m.Response.BodyForClipboard()); err != nil {
		m.StatusBar = "Clipboard unavailable"
		return nil
	}
	m.StatusBar = "Response body copied"
	return nil
}

func (m *MainScreen) closeFile() {
	if m.ActiveFile == "" {
		return
	}
	m.saveAndEnqueue()
	m.ActiveFile = ""
	m.ViewMode = ViewRunner
	m.Focus = FocusExplorerPane
	m.LastFocus = FocusURLBar
	m.setExplorerFocused(true)
	m.blurAll()
	m.URLBar.CloseDropdown()
	m.Response.CloseSearch()
	m.Response.SetResponse(0, "", 0, 0, false, "")
}
