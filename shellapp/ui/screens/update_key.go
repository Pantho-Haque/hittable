package screens

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/ui/components/explorer"
	"github.com/hittable/shellapp/ui/components/palette"
	"github.com/hittable/shellapp/ui/components/requesteditor"
	"github.com/hittable/shellapp/ui/components/texteditor"
)

func (m *MainScreen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.StatusBar = ""
	key := msg.String()
	if keyLog != nil {
		fmt.Fprintf(keyLog, "%q type=%d alt=%v\n", key, msg.Type, msg.Alt)
	}

	// Navbar shortcuts win over every panel, so switching tabs always works.
	if m.navKey(key) {
		return m, nil
	}

	// Help overlay swallows everything except its own toggles.
	if m.ShowHelp {
		switch key {
		case "?", "esc", "q":
			m.ShowHelp = false
		case "down", "j":
			m.HelpScroll++
		case "up", "k":
			m.HelpScroll--
		case "pgdown", " ":
			m.HelpScroll += 10
		case "pgup":
			m.HelpScroll -= 10
		case "home", "g":
			m.HelpScroll = 0
		case "end", "G":
			m.HelpScroll = 1 << 20 // clamped when rendering
		case "ctrl+c":
			m.saveAndEnqueue()
			return m, tea.Quit
		}
		return m, nil
	}

	if m.Palette.Open {
		switch key {
		case "ctrl+c":
			m.saveAndEnqueue()
			return m, tea.Quit
		default:
			m.Palette.HandleKey(msg)
		}
		return m, nil
	}
	if m.GitOpen {
		if key == "ctrl+c" {
			if m.Git.HasSelection() {
				m.Git.CopySelection()
				return m, nil
			}
			m.saveAndEnqueue()
			return m, tea.Quit
		}
		return m.handleGitKey(msg)
	}

	// Integrated terminal owns every key while focused, except the two that
	// leave it. (ctrl+c goes to the shell, as in VS Code.)
	if m.TermFocused {
		switch key {
		case "shift+up", "shift+pgup":
			m.Term.Scroll(m.Term.Rows / 2)
			return m, nil
		case "shift+down", "shift+pgdown":
			m.Term.Scroll(-m.Term.Rows / 2)
			return m, nil
		case "ctrl+c":
			// Copies a mouse selection, otherwise interrupts the shell.
			if n := m.Term.CopySelection(); n > 0 {
				m.Term.ClearSelection()
				m.StatusBar = fmt.Sprintf("copied %d line(s)", n)
				return m, nil
			}
			m.Term.SendKey(msg)
			return m, nil
		case "ctrl+b":
			m.TermFocused = false
			m.LastFocus = m.Focus
			m.Focus = FocusExplorerPane
			m.setExplorerFocused(true)
		default:
			m.Term.ClearSelection()
			m.Term.SendKey(msg)
		}
		return m, nil
	}

	// Global shortcuts, regardless of focus.
	switch key {
	case "ctrl+g":
		if m.focusedEditor() == nil { // editors use ctrl+g for go-to-line
			m.toggleGit()
			return m, nil
		}
	case "ctrl+c":
		if ed := m.focusedEditor(); ed != nil && ed.HasSelection() {
			ed.Copy()
			return m, nil
		}
		m.saveAndEnqueue() // app.Cleanup flushes the queue after Run returns
		return m, tea.Quit
	case "ctrl+b":
		return m, m.toggleExplorerFocus()
	case "ctrl+s":
		m.saveAndEnqueue()
		m.WriteQueue.FlushNow()
		m.StatusBar = "Saved"
		return m, nil
	case "ctrl+r", "ctrl+enter":
		if m.ActiveFile != "" && !m.inTextPrompt() {
			m.saveAndEnqueue()
			return m, m.sendRequestAsync()
		}
	case "ctrl+t":
		if m.ActiveFile != "" && !m.inTextPrompt() {
			if m.isMarkdown() {
				m.setMdMode((m.MdMode + 1) % 3)
			} else {
				m.toggleViewMode()
			}
			return m, nil
		}
	case "ctrl+y":
		if m.Focus == FocusResponse && !m.ExplorerFocused {
			return m, m.copyResponseBody()
		}
		if m.Focus == FocusTextEditor && !m.ExplorerFocused {
			return m, m.handleTextEditorKey(msg) // redo
		}
		return m, m.handleCopyCurl()
	}

	// The code editor owns its keys (undo/redo, find, go-to-line, tab).
	if m.Focus == FocusTextEditor && !m.ExplorerFocused {
		switch key {
		case "esc":
			if m.TextEd.PromptOpen() {
				return m, m.handleTextEditorKey(msg)
			}
			return m.handleEsc()
		case "shift+tab":
			if m.isMarkdown() && m.MdMode == MdSplit {
				m.blurAll()
				m.Focus = FocusPreview
				return m, nil
			}
			return m.handleShiftTab()
		}
		cmd := m.handleTextEditorKey(msg)
		if m.isMarkdown() && m.MdMode == MdSplit {
			// Keep the preview near the cursor while editing.
			if n := m.TextEd.TextArea.LineCount(); n > 1 {
				m.Preview.SetContent(m.TextEd.GetContent())
				m.Preview.ScrollToFraction(float64(m.TextEd.GetCursorRow()) / float64(n-1))
			}
		}
		return m, cmd
	}

	if m.ExplorerFocused {
		return m.handleExplorerKey(msg)
	}
	if m.Focus == FocusMethodDropdown {
		return m, m.handleDropdownKey(msg)
	}
	if m.Focus == FocusResponse && m.Response.SearchOpen {
		return m, m.handleResponseSearchKey(msg)
	}

	if m.Focus == FocusPreview {
		switch key {
		case "esc":
			return m.handleEsc()
		case "tab", "shift+tab":
			if m.MdMode == MdSplit {
				m.Focus = FocusTextEditor
				m.focusCurrent()
			}
			return m, nil
		}
		m.Preview.HandleKey(msg)
		return m, nil
	}

	switch key {
	case "ctrl+f":
		if m.ActiveFile != "" && m.ViewMode == ViewRunner {
			m.blurAll()
			m.Response.OpenSearch()
			m.Focus = FocusResponse
		}
		return m, nil
	case "esc":
		return m.handleEsc()
	case "tab":
		return m.handleTab()
	case "shift+tab":
		return m.handleShiftTab()
	case "alt+1", "alt+2", "alt+3":
		if m.ViewMode == ViewRunner && m.ActiveFile != "" {
			m.ActiveTab = requesteditor.Tab(int(key[4] - '1'))
			m.Focus = FocusBody
			m.focusCurrent()
		}
		return m, nil
	}

	switch m.Focus {
	case FocusURLBar:
		switch key {
		case "enter":
			m.URLBar.ToggleDropdown()
			m.Focus = FocusMethodDropdown
			return m, nil
		case "ctrl+left", "ctrl+right":
			dir := 1
			if key == "ctrl+left" {
				dir = -1
			}
			m.URLBar.CycleMethod(dir)
			m.saveAndEnqueueDebounced()
			return m, nil
		}
		cmd := m.URLBar.Update(msg)
		m.saveAndEnqueueDebounced()
		return m, cmd
	case FocusTabBar:
		m.handleTabBarKey(msg)
		return m, nil
	case FocusBody:
		return m, m.handleBodyKey(msg)
	case FocusTextEditor:
		return m, m.handleTextEditorKey(msg)
	case FocusResponse:
		return m, m.Response.Update(msg)
	}
	return m, nil
}

// inTextPrompt reports whether the explorer is capturing typed text, so
// ctrl+ shortcuts that could collide with its inline prompts are ignored.
func (m *MainScreen) inTextPrompt() bool {
	return m.ExplorerFocused && (m.Explorer.PromptKind != explorer.PromptNone || m.Explorer.DeletePending || m.Explorer.MenuOpen)
}

func (m *MainScreen) toggleExplorerFocus() tea.Cmd {
	if m.ExplorerHidden {
		m.ExplorerHidden = false
		m.SetSize(m.Width, m.Height)
		m.LastFocus = m.Focus
		m.blurAll()
		m.Focus = FocusExplorerPane
		m.setExplorerFocused(true)
		return nil
	}
	if m.ExplorerFocused {
		if m.ActiveFile == "" {
			return nil
		}
		m.setExplorerFocused(false)
		m.Focus = m.LastFocus
		if m.Focus == FocusExplorerPane || m.Focus == FocusMethodDropdown {
			m.Focus = FocusURLBar
			if m.ViewMode == ViewText {
				m.Focus = FocusTextEditor
			}
		}
		m.focusCurrent()
	} else {
		m.LastFocus = m.Focus
		m.blurAll()
		m.URLBar.CloseDropdown()
		m.Focus = FocusExplorerPane
		m.setExplorerFocused(true)
	}
	return nil
}

func (m *MainScreen) setExplorerFocused(v bool) {
	m.ExplorerFocused = v
	m.Explorer.Focused = v
}

// focusCurrent focuses the component matching m.Focus.
func (m *MainScreen) focusCurrent() {
	m.blurAll()
	m.setExplorerFocused(false)
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
	case FocusTextEditor:
		m.TextEd.Focus()
	}
}

func (m *MainScreen) blurAll() {
	m.URLBar.Blur()
	m.Params.Blur()
	m.Headers.Blur()
	m.Body.Blur()
	m.TextEd.Blur()
}

// handleExplorerKey handles keys while the explorer has focus. The explorer
// owns menu / prompt / delete-confirm state; this dispatches shortcuts and
// applies confirmed prompts.
func (m *MainScreen) handleExplorerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.Explorer.MenuOpen {
		m.Explorer.HandleKey(msg)
		return m, nil
	}
	if m.Explorer.DeletePending {
		if msg.String() == "y" || msg.String() == "Y" {
			if p, ok := m.Explorer.ConfirmDelete(); ok {
				return m, func() tea.Msg { return deleteConfirmMsg{path: p} }
			}
		}
		m.Explorer.HandleKey(msg)
		return m, nil
	}
	if m.Explorer.PromptKind != explorer.PromptNone {
		if msg.String() == "enter" {
			isRename := m.Explorer.PromptKind == explorer.PromptRename
			a, b, isDir, ok := m.Explorer.ConfirmPrompt()
			if !ok {
				return m, nil
			}
			if isRename {
				return m, func() tea.Msg { return renameConfirmMsg{oldPath: a, newPath: b} }
			}
			return m, func() tea.Msg { return addConfirmMsg{parent: a, name: b, isDir: isDir} }
		}
		m.Explorer.HandleKey(msg)
		return m, nil
	}

	switch msg.String() {
	case "?":
		m.toggleHelp()
	case "g":
		m.toggleGit()
	case "/":
		m.openPalette(palette.ModeFiles)
	case "ctrl+n":
		m.Explorer.BeginNewFile()
	case "ctrl+shift+n", "ctrl+f":
		m.Explorer.BeginNewFolder()
	case "ctrl+e":
		m.Explorer.BeginRename()
	case "ctrl+d", "delete":
		m.Explorer.BeginDelete()
	case "esc":
		if m.ActiveFile != "" {
			m.closeFile()
		}
	case "tab", "right":
		if m.ActiveFile != "" {
			m.toggleExplorerFocus()
		}
	default:
		m.Explorer.HandleKey(msg)
		if msg.String() == "r" {
			m.refreshGit(true)
		}
	}
	return m, nil
}

func (m *MainScreen) handleDropdownKey(msg tea.KeyMsg) tea.Cmd {
	if m.URLBar.HandleDropdownKey(msg) && !m.URLBar.DropdownOpen {
		m.Focus = FocusURLBar
		m.focusCurrent()
		m.saveAndEnqueueDebounced()
	}
	return nil
}

func (m *MainScreen) handleResponseSearchKey(msg tea.KeyMsg) tea.Cmd {
	return m.Response.Update(msg)
}

func (m *MainScreen) handleTabBarKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "left", "h":
		m.ActiveTab = (m.ActiveTab + 2) % 3
	case "right", "l":
		m.ActiveTab = (m.ActiveTab + 1) % 3
	case "1":
		m.ActiveTab = requesteditor.TabParams
	case "2":
		m.ActiveTab = requesteditor.TabHeaders
	case "3":
		m.ActiveTab = requesteditor.TabBody
	case "enter", "down":
		m.Focus = FocusBody
		m.focusCurrent()
	case "up":
		m.Focus = FocusURLBar
		m.focusCurrent()
	}
}

func (m *MainScreen) handleBodyKey(msg tea.KeyMsg) tea.Cmd {
	if msg.String() == "ctrl+l" && m.ActiveTab == requesteditor.TabBody {
		if m.Body.FormatJSON() {
			m.StatusBar = "Body formatted"
		} else {
			m.StatusBar = "Body is not valid JSON"
		}
		m.saveAndEnqueueDebounced()
		return nil
	}
	var cmd tea.Cmd
	switch m.ActiveTab {
	case requesteditor.TabParams:
		cmd = m.Params.Update(msg)
	case requesteditor.TabHeaders:
		cmd = m.Headers.Update(msg)
	case requesteditor.TabBody:
		cmd = m.Body.Update(msg)
	}
	m.saveAndEnqueueDebounced()
	return cmd
}

func (m *MainScreen) handleTextEditorKey(msg tea.KeyMsg) tea.Cmd {
	cmd := m.TextEd.Update(msg)
	m.saveAndEnqueueDebounced()
	return cmd
}

// focusedEditor returns the code editor that currently has keyboard focus.
func (m *MainScreen) focusedEditor() *texteditor.TextEditor {
	if m.ExplorerFocused || m.ActiveFile == "" {
		return nil
	}
	switch m.Focus {
	case FocusTextEditor:
		return m.TextEd
	case FocusBody:
		switch m.ActiveTab {
		case requesteditor.TabParams:
			return m.Params.TextEditor
		case requesteditor.TabHeaders:
			return m.Headers.TextEditor
		}
		return m.Body.TextEditor
	}
	return nil
}

// toggleTerminal cycles: closed → open+focused → closed. If the panel is open
// but unfocused, it takes focus instead.
func (m *MainScreen) toggleTerminal() {
	switch {
	case !m.Term.Open:
		m.closePanels()
		m.Term.Open = true
		m.SetSize(m.Width, m.Height)
		if err := m.Term.Start(); err != nil {
			m.StatusBar = "terminal: " + err.Error()
			m.Term.Open = false
			m.SetSize(m.Width, m.Height)
			return
		}
		m.focusTerminal()
	case m.TermFocused:
		m.Term.Open = false
		m.TermFocused = false
		m.Term.Focused = false
		m.SetSize(m.Width, m.Height)
		if m.ActiveFile != "" && !m.ExplorerFocused {
			m.focusCurrent()
		} else {
			m.setExplorerFocused(true)
		}
	default:
		m.focusTerminal()
	}
}

func (m *MainScreen) focusTerminal() {
	m.blurAll()
	m.setExplorerFocused(false)
	m.URLBar.CloseDropdown()
	m.TermFocused = true
	m.Term.Focused = true
	m.StatusBar = ""
}

// leaveTerminal drops terminal focus (the caller sets the new focus).
func (m *MainScreen) leaveTerminal() {
	m.TermFocused = false
	m.Term.Focused = false
}

// keyLog, when HITTABLE_KEYLOG is set to a path, records every key name the
// terminal delivers — handy for finding what a given chord arrives as.
var keyLog = func() *os.File {
	p := os.Getenv("HITTABLE_KEYLOG")
	if p == "" {
		return nil
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	return f
}()

// navKey handles the navbar tab shortcuts. Returns true if consumed.
func (m *MainScreen) navKey(key string) bool {
	switch key {
	case "f5", "alt+g":
		m.toggleGit()
	case "alt+f":
		if m.Palette.Open && m.Palette.Mode == palette.ModeGrep {
			m.Palette.Close()
		} else {
			m.openPalette(palette.ModeGrep)
		}
	case "ctrl+p":
		if m.TermFocused {
			return false // shell history
		}
		if m.Palette.Open && m.Palette.Mode == palette.ModeFiles {
			m.Palette.Close()
		} else {
			m.openPalette(palette.ModeFiles)
		}
	case "ctrl+j", "ctrl+@", "ctrl+`":
		m.toggleTerminal()
	case "f1":
		m.toggleHelp()
	case "alt+b":
		m.toggleExplorer()
	default:
		return false
	}
	return true
}

// closePanels hides every navbar panel (Git, Find, Terminal, Help). The
// navbar buttons behave as exclusive tabs: opening one closes the rest.
func (m *MainScreen) closePanels() {
	m.GitOpen = false
	m.ShowHelp = false
	m.Palette.Close()
	if m.Term.Open {
		m.Term.Open = false
		m.leaveTerminal()
		m.SetSize(m.Width, m.Height)
	}
}

// openPalette shows the find-file / live-grep overlay.
func (m *MainScreen) openPalette(mode palette.Mode) {
	m.closePanels()
	m.blurAll()
	m.setExplorerFocused(false)
	m.Palette.Show(mode)
}

// toggleHelp shows / hides the keyboard reference.
func (m *MainScreen) toggleHelp() {
	m.HelpScroll = 0
	if m.ShowHelp {
		m.ShowHelp = false
		return
	}
	m.closePanels()
	m.ShowHelp = true
}
