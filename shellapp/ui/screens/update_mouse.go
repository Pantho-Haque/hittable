package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/ui/components/gitpanel"
	"github.com/hittable/shellapp/ui/components/palette"
	"github.com/hittable/shellapp/ui/components/requesteditor"
)

func (m *MainScreen) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	x := msg.X

	// bubbletea reports a drag as a left-button event with a motion action.
	press := msg.Type == tea.MouseLeft && msg.Action == tea.MouseActionPress
	drag := msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonLeft

	// Hover feedback: one lookup for every clickable zone in the app, so the
	// branches below only have to deal with clicks.
	if msg.Type == tea.MouseMotion {
		m.HoverZone = m.findZoneAt(msg)
	}

	if msg.Type == tea.MouseRelease {
		m.Dragging = false
		m.Git.EndDrag()
		if ed := m.focusedEditor(); ed != nil {
			ed.Update(m.editorRelative(msg))
		}
		return m, nil
	}
	if drag && m.Dragging {
		m.ExplorerWidth += x - m.DragStartX
		m.DragStartX = x
		m.SetSize(m.Width, m.Height) // clamps width and resizes children
		return m, nil
	}
	if drag {
		// Text selection drag inside the focused editor, terminal or git pane.
		if ed := m.focusedEditor(); ed != nil {
			return m, ed.Update(m.editorRelative(msg))
		}
		if z := m.Zones.Get("term_view"); m.Term.Open && z != nil && z.InBounds(msg) {
			m.Term.SelectTo(msg.X-z.StartX, msg.Y-z.StartY)
			return m, nil
		}
		if m.GitOpen {
			if z := m.Zones.Get("git_panel"); z != nil && z.InBounds(msg) {
				rel := msg
				rel.X -= z.StartX
				rel.Y -= z.StartY
				m.Git.HandleMouse(rel)
			}
		}
		return m, nil
	}
	// Top bar buttons.
	if msg.Y == 0 {
		if press {
			switch m.findTopZone(msg) {
			case "top_sidebar":
				m.toggleExplorer()
			case "top_sync":
				m.Git.SyncAction()
			case "top_git", "top_branch":
				m.toggleGit()
			case "top_find":
				if m.Palette.Open {
					m.Palette.Close()
				} else {
					m.openPalette(palette.ModeFiles)
				}
			case "top_term":
				m.toggleTerminal()
			case "top_help":
				m.toggleHelp()
			}
		}
		return m, nil
	}

	// Separator column between explorer and main pane.
	if press && !m.explorerHidden() && x == m.ExplorerWidth {
		m.Dragging = true
		m.DragStartX = x
		return m, nil
	}

	if !m.explorerHidden() && x < m.ExplorerWidth {
		if m.ShowHelp && msg.Type == tea.MouseLeft {
			m.ShowHelp = false
		}
		if msg.Type == tea.MouseLeft || msg.Type == tea.MouseRight {
			m.leaveTerminal()
			m.StatusBar = ""
			if !m.ExplorerFocused {
				m.LastFocus = m.Focus
				m.blurAll()
			}
			m.setExplorerFocused(true)
			m.Focus = FocusExplorerPane
		}
		if press || msg.Type == tea.MouseRight {
			m.GitOpen = false
			m.Palette.Close()
		}
		em := msg
		em.Y--                     // explorer starts under the top bar
		m.Explorer.HandleMouse(em) // openFileRaw flips focus back for files
		return m, nil
	}
	m.Explorer.HoverRow = -1

	// ---- terminal strip / panel (usable with no file open) ----
	if m.Term.Open && (msg.Type == tea.MouseWheelUp || msg.Type == tea.MouseWheelDown) {
		if z := m.Zones.Get("term_view"); z != nil && z.InBounds(msg) {
			if msg.Type == tea.MouseWheelUp {
				m.Term.Scroll(3)
			} else {
				m.Term.Scroll(-3)
			}
			return m, nil
		}
	}
	if press {
		if z := m.Zones.Get("term_strip"); z != nil && z.InBounds(msg) {
			m.toggleTerminal()
			return m, nil
		}
		if z := m.Zones.Get("term_view"); m.Term.Open && z != nil && z.InBounds(msg) {
			m.focusTerminal()
			m.Term.SelectStart(msg.X-z.StartX, msg.Y-z.StartY)
			return m, nil
		}
	}

	// ---- find palette ----
	if m.Palette.Open {
		if z := m.Zones.Get("palette"); z != nil && z.InBounds(msg) {
			rel := msg
			rel.X -= z.StartX
			rel.Y -= z.StartY
			m.Palette.HandleMouse(rel)
		} else if press {
			m.Palette.Close()
		} else if msg.Type == tea.MouseMotion {
			m.Palette.HoverRow = -1
		}
		return m, nil
	}

	// ---- git panel ----
	if m.GitOpen {
		if z := m.Zones.Get("git_panel"); z != nil && z.InBounds(msg) {
			if press {
				if tz := m.Zones.Get("git_diffmode"); tz != nil && tz.InBounds(msg) {
					m.Git.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
					return m, nil
				}
				if tz := m.Zones.Get("git_wrap"); tz != nil && tz.InBounds(msg) {
					m.Git.ToggleWrap()
					return m, nil
				}
				if tz := m.Zones.Get("git_sync"); tz != nil && tz.InBounds(msg) {
					m.Git.SyncAction()
					return m, nil
				}
				for i := range gitpanel.SectionNames {
					if tz := m.Zones.Get(fmt.Sprintf("git_tab_%d", i)); tz != nil && tz.InBounds(msg) {
						m.Git.Section = gitpanel.Section(i)
						m.Git.Cursor = 0
						m.Git.Refresh()
						return m, nil
					}
				}
			}
			rel := msg
			rel.X -= z.StartX
			rel.Y -= z.StartY
			m.Git.HandleMouse(rel)
		} else if msg.Type == tea.MouseMotion {
			m.Git.HoverRow = -1
		}
		return m, nil
	}

	// ---- main pane ----
	if m.ShowHelp {
		switch msg.Type {
		case tea.MouseWheelUp:
			m.HelpScroll -= 3
		case tea.MouseWheelDown:
			m.HelpScroll += 3
		case tea.MouseLeft:
			m.ShowHelp = false
		}
		return m, nil
	}
	if m.ActiveFile == "" {
		return m, nil
	}
	switch msg.Type {
	case tea.MouseWheelLeft, tea.MouseWheelRight:
		if ed := m.focusedEditor(); ed != nil {
			return m, ed.Update(m.editorRelative(msg))
		}
		return m, nil
	case tea.MouseWheelUp, tea.MouseWheelDown:
		if z := m.Zones.Get("md_pane"); z != nil && !z.IsZero() && z.InBounds(msg) {
			if msg.Type == tea.MouseWheelUp {
				m.Preview.Scroll(-3)
			} else {
				m.Preview.Scroll(3)
			}
			return m, nil
		}
		if m.ViewMode == ViewText {
			return m, m.TextEd.Update(m.editorRelative(msg))
		}
		if m.Zones.Get("response").InBounds(msg) {
			m.Response.Scroll(msg.Type == tea.MouseWheelUp)
		} else if m.Zones.Get("editor").InBounds(msg) {
			if m.Focus != FocusBody {
				m.Focus = FocusBody
				m.focusCurrent()
			}
			return m, m.handleBodyMouse(m.editorRelative(msg))
		}
		return m, nil
	case tea.MouseMotion:
		return m, nil
	case tea.MouseLeft:
		if !press {
			return m, nil
		}
	default:
		return m, nil
	}

	m.StatusBar = ""
	m.setExplorerFocused(false)
	m.leaveTerminal()
	if m.URLBar.DropdownOpen {
		zoneID := m.findZoneAt(msg)
		m.URLBar.CloseDropdown()
		m.Focus = FocusURLBar
		m.URLBar.Focus()
		if strings.HasPrefix(zoneID, "method_") {
			method := strings.TrimPrefix(zoneID, "method_")
			for i, met := range requesteditor.Methods {
				if met == method {
					idx := i
					return m, func() tea.Msg { return methodSelectMsg{idx: idx, method: method} }
				}
			}
		}
		return m, nil
	}

	switch zoneID := m.findZoneAt(msg); zoneID {
	case "tab_Params":
		return m, func() tea.Msg { return tabClickMsg{tab: requesteditor.TabParams} }
	case "tab_Headers":
		return m, func() tea.Msg { return tabClickMsg{tab: requesteditor.TabHeaders} }
	case "tab_Body":
		return m, func() tea.Msg { return tabClickMsg{tab: requesteditor.TabBody} }
	case "toggle_runner":
		return m, func() tea.Msg { return toggleViewMsg{mode: ViewRunner} }
	case "toggle_text":
		return m, func() tea.Msg { return toggleViewMsg{mode: ViewText} }
	case "search_icon":
		return m, func() tea.Msg { return searchIconClickMsg{} }
	case "method_badge":
		m.URLBar.ToggleDropdown()
		m.Focus = FocusMethodDropdown
		return m, nil
	case "send_btn":
		m.saveAndEnqueue()
		return m, m.sendRequestAsync()
	case "fold_all":
		m.TextEd.ToggleFoldAll()
		return m, nil
	case "md_text":
		m.setMdMode(MdText)
		return m, nil
	case "md_preview":
		m.setMdMode(MdPreview)
		return m, nil
	case "md_split":
		m.setMdMode(MdSplit)
		return m, nil
	case "md_pane":
		m.blurAll()
		m.Focus = FocusPreview
		return m, nil
	case "urlbar":
		m.Focus = FocusURLBar
		m.focusCurrent()
		if z := m.Zones.Get("urlbar"); !z.IsZero() {
			m.URLBar.ClickAt(msg.X - z.StartX - 1)
		}
		return m, nil
	case "resp_mode":
		m.Response.ShowHeaders = !m.Response.ShowHeaders
		m.Response.ScrollY = 0
		m.Focus = FocusResponse
		m.blurAll()
		return m, nil
	case "response":
		m.Focus = FocusResponse
		m.blurAll()
		return m, nil
	case "editor":
		if m.ViewMode == ViewText {
			m.Focus = FocusTextEditor
			m.focusCurrent()
			return m, m.TextEd.Update(m.editorRelative(msg))
		}
		m.Focus = FocusBody
		m.focusCurrent()
		return m, m.handleBodyMouse(m.editorRelative(msg))
	}
	return m, nil
}

func (m *MainScreen) handleBodyMouse(msg tea.MouseMsg) tea.Cmd {
	switch m.ActiveTab {
	case requesteditor.TabParams:
		return m.Params.Update(msg)
	case requesteditor.TabHeaders:
		return m.Headers.Update(msg)
	}
	return m.Body.Update(msg)
}

// findZoneAt names the clickable zone under the mouse. Zones are tested
// innermost first — the small buttons before the panes containing them — so
// the first hit wins. It drives both click dispatch and hover styling, which
// is what keeps the two from disagreeing about what the mouse is over.
func (m *MainScreen) findZoneAt(msg tea.MouseMsg) string {
	ids := topZoneIDs()
	switch {
	case m.Palette.Open: // its rows are hit-tested inside the component
	case m.GitOpen:
		ids = append(ids, "git_diffmode", "git_wrap", "git_sync")
		for i := range gitpanel.SectionNames {
			ids = append(ids, fmt.Sprintf("git_tab_%d", i))
		}
	default:
		ids = append(ids, "tab_Params", "tab_Headers", "tab_Body", "toggle_runner", "toggle_text",
			"search_icon", "resp_mode", "method_badge", "send_btn")
		if m.URLBar.DropdownOpen {
			for _, method := range requesteditor.Methods {
				ids = append(ids, "method_"+method)
			}
		}
		ids = append(ids, "fold_all", "md_text", "md_preview", "md_split", "md_pane", "urlbar", "response", "editor")
	}
	ids = append(ids, "term_strip")
	for _, id := range ids {
		if z := m.Zones.Get(id); z != nil && !z.IsZero() && z.InBounds(msg) {
			return id
		}
	}
	return ""
}

// editorRelative converts screen coordinates to the code editor's content
// cell grid (inside its border).
func (m *MainScreen) editorRelative(msg tea.MouseMsg) tea.MouseMsg {
	z := m.Zones.Get("editor")
	if z.IsZero() {
		return msg
	}
	msg.X -= z.StartX + 1
	msg.Y -= z.StartY + 1
	return msg
}

func topZoneIDs() []string {
	return []string{"top_sidebar", "top_git", "top_find", "top_term", "top_help", "top_branch", "top_sync"}
}

func (m *MainScreen) findTopZone(msg tea.MouseMsg) string {
	for _, id := range topZoneIDs() {
		if z := m.Zones.Get(id); z != nil && !z.IsZero() && z.InBounds(msg) {
			return id
		}
	}
	return ""
}
