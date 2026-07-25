package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/ui/components/requesteditor"
)

func (m *MainScreen) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	x := msg.X
	y := msg.Y

	if msg.Type == tea.MouseRelease {
		m.Dragging = false
		return m, nil
	}

	if msg.Type == tea.MouseMotion && m.Dragging {
		delta := x - m.DragStartX
		m.ExplorerWidth += delta
		m.DragStartX = x
		if m.ExplorerWidth < 15 {
			m.ExplorerWidth = 15
		}
		if m.ExplorerWidth > m.Width-20 {
			m.ExplorerWidth = m.Width - 20
		}
		m.MainWidth = m.Width - m.ExplorerWidth - 1
		m.Explorer.SetSize(m.ExplorerWidth, m.Height-2)
		m.URLBar.SetSize(m.MainWidth - 4)
		m.Params.SetSize(m.MainWidth-4, m.Height/3)
		m.Headers.SetSize(m.MainWidth-4, m.Height/3)
		m.Body.SetSize(m.MainWidth-4, m.Height/3)
		m.Response.SetSize(m.MainWidth-4, m.Height/3)
		m.TextEd.SetSize(m.MainWidth-4, m.Height-4)
		return m, nil
	}

	if msg.Type == tea.MouseMotion {
		zoneID := m.findZoneAt(msg)
		m.HoverZone = zoneID
		m.updateExplorerHover(y)
		return m, nil
	}

	if msg.Type == tea.MouseLeft {
		separatorX := m.ExplorerWidth
		if x == separatorX || x == separatorX+1 {
			m.Dragging = true
			m.DragStartX = x
			return m, nil
		}

		zoneID := m.findZoneAt(msg)

		if m.URLBar.DropdownOpen && !strings.HasPrefix(zoneID, "method_") {
			m.URLBar.CloseDropdown()
			m.Focus = FocusURLBar
			m.URLBar.Focus()
		}

		if zoneID == "" {
			if x < m.ExplorerWidth {
				m.ExplorerFocused = true
				m.URLBar.Blur()
				m.Params.Blur()
				m.Headers.Blur()
				m.Body.Blur()
				m.TextEd.Blur()
				m.Focus = FocusExplorerPane
			}
			return m, nil
		}

		if strings.HasPrefix(zoneID, "explorer_") {
			var idx int
			fmt.Sscanf(strings.TrimPrefix(zoneID, "explorer_"), "%d", &idx)
			return m, func() tea.Msg { return explorerClickMsg{idx: idx} }
		}
		if zoneID == "tab_Params" {
			return m, func() tea.Msg { return tabClickMsg{tab: requesteditor.TabParams} }
		}
		if zoneID == "tab_Headers" {
			return m, func() tea.Msg { return tabClickMsg{tab: requesteditor.TabHeaders} }
		}
		if zoneID == "tab_Body" {
			return m, func() tea.Msg { return tabClickMsg{tab: requesteditor.TabBody} }
		}
		if zoneID == "toggle_runner" {
			return m, func() tea.Msg { return toggleViewMsg{mode: ViewRunner} }
		}
		if zoneID == "toggle_text" {
			return m, func() tea.Msg { return toggleViewMsg{mode: ViewText} }
		}
		if strings.HasPrefix(zoneID, "method_") {
			method := strings.TrimPrefix(zoneID, "method_")
			for i, met := range requesteditor.Methods {
				if met == method {
					idx := i
					return m, func() tea.Msg { return methodSelectMsg{idx: idx, method: method} }
				}
			}
		}
		if zoneID == "search_icon" {
			return m, func() tea.Msg { return searchIconClickMsg{} }
		}
		if zoneID == "method_badge" {
			m.URLBar.ToggleDropdown()
			if m.URLBar.DropdownOpen {
				m.Focus = FocusMethodDropdown
			}
			return m, nil
		}
	}

	if msg.Type == tea.MouseRight {
		if x < m.ExplorerWidth {
			clickRow := y - 1 + m.Explorer.GetScrollStart()
			m.ExplorerFocused = true
			m.Focus = FocusExplorerPane
			if clickRow >= 0 && clickRow < len(m.Explorer.Visible) {
				m.Explorer.Cursor = clickRow
				m.Explorer.OpenContextMenuAtRow(clickRow)
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *MainScreen) updateExplorerHover(y int) {
	if !m.ExplorerFocused {
		return
	}
	row := y - 1 + m.Explorer.GetScrollStart()
	m.Explorer.HoverAtRow(row)
}

func (m *MainScreen) findZoneAt(msg tea.MouseMsg) string {
	if m.URLBar.DropdownOpen {
		for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
			id := fmt.Sprintf("method_%s", method)
			if !m.Zones.Get(id).IsZero() && m.Zones.Get(id).InBounds(msg) {
				return id
			}
		}
	}

	nodeCount := len(m.Explorer.Visible)
	if nodeCount > 500 {
		nodeCount = 500
	}
	for i := 0; i < nodeCount; i++ {
		id := fmt.Sprintf("explorer_%d", i)
		if !m.Zones.Get(id).IsZero() && m.Zones.Get(id).InBounds(msg) {
			return id
		}
	}
	for _, id := range []string{"tab_Params", "tab_Headers", "tab_Body", "toggle_runner", "toggle_text", "search_icon", "method_badge"} {
		if !m.Zones.Get(id).IsZero() && m.Zones.Get(id).InBounds(msg) {
			return id
		}
	}
	if !m.URLBar.DropdownOpen {
		for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
			id := fmt.Sprintf("method_%s", method)
			if !m.Zones.Get(id).IsZero() && m.Zones.Get(id).InBounds(msg) {
				return id
			}
		}
	}
	return ""
}

func (m *MainScreen) handleExplorerClick(idx int) {
	m.ExplorerFocused = true
	m.URLBar.Blur()
	m.Params.Blur()
	m.Headers.Blur()
	m.Body.Blur()
	m.TextEd.Blur()
	m.Focus = FocusExplorerPane
	m.Explorer.ClickAtRow(idx)
}
