// Package explorer implements the update logic for the explorer component.
package explorer

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (e *ExplorerComponent) Update(msg tea.Msg) tea.Cmd {
	if e.Renaming {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				return func() tea.Msg { return RenameConfirmMsg{} }
			case "esc":
				e.Renaming = false
				return nil
			}
		}
		var cmd tea.Cmd
		e.RenameInput, cmd = e.RenameInput.Update(msg)
		return cmd
	}
	if e.AddingFile || e.AddingFolder {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				return func() tea.Msg { return AddConfirmMsg{} }
			case "esc":
				e.AddingFile = false
				e.AddingFolder = false
				return nil
			}
		}
		var cmd tea.Cmd
		e.AddInput, cmd = e.AddInput.Update(msg)
		return cmd
	}
	if e.Deleting {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "Y":
				return func() tea.Msg { return DeleteConfirmMsg{} }
			case "n", "N", "esc":
				e.Deleting = false
				return nil
			}
		}
		return nil
	}
	if e.ContextMenuOpen {
		if msg, ok := msg.(tea.KeyMsg); ok {
			cmd := e.handleContextMenuInput(msg)
			return cmd
		}
		if msg, ok := msg.(tea.MouseMsg); ok {
			if msg.Type == tea.MouseLeft {
				e.CloseContextMenu()
				return nil
			}
		}
		return nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !e.Focused {
			return nil
		}
		switch {
		case key.Matches(msg, e.keyMap().Up):
			if e.Cursor > 0 {
				e.Cursor--
			}
		case key.Matches(msg, e.keyMap().Down):
			if e.Cursor < len(e.Visible)-1 {
				e.Cursor++
			}
		case key.Matches(msg, e.keyMap().VimUpInExplorer):
			if e.Cursor > 0 {
				e.Cursor--
			}
		case key.Matches(msg, e.keyMap().VimDownInExplorer):
			if e.Cursor < len(e.Visible)-1 {
				e.Cursor++
			}
		case key.Matches(msg, e.keyMap().Enter):
			e.ClickAtRow(e.Cursor)
		}
	}
	return nil
}

func (e *ExplorerComponent) keyMap() struct {
	Up, Down, VimUpInExplorer, VimDownInExplorer, Enter key.Binding
} {
	return struct {
		Up, Down, VimUpInExplorer, VimDownInExplorer, Enter key.Binding
	}{
		Up:                key.NewBinding(key.WithKeys("up")),
		Down:              key.NewBinding(key.WithKeys("down")),
		VimUpInExplorer:   key.NewBinding(key.WithKeys("k")),
		VimDownInExplorer: key.NewBinding(key.WithKeys("j")),
		Enter:             key.NewBinding(key.WithKeys("enter")),
	}
}
