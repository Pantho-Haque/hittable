package keymap

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit        key.Binding
	Explorer    key.Binding
	Send        key.Binding
	Save        key.Binding
	Tab         key.Binding
	ShiftTab    key.Binding
	Esc         key.Binding
	Up          key.Binding
	Down        key.Binding
	Enter       key.Binding
	VimUp       key.Binding
	VimDown     key.Binding
	Help        key.Binding
	Search      key.Binding
	ContextMenu key.Binding
	NewFile     key.Binding
	NewFolder   key.Binding
	Rename      key.Binding
	Delete      key.Binding
	ToggleView  key.Binding
	CopyCurl    key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit:        key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
		Explorer:    key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("ctrl+b", "explorer")),
		Send:        key.NewBinding(key.WithKeys("ctrl+enter"), key.WithHelp("ctrl+⏎", "send")),
		Save:        key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save")),
		Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
		ShiftTab:    key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev")),
		Esc:         key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Up:          key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		Down:        key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
		Enter:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "open")),
		VimUp:       key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "up")),
		VimDown:     key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "down")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Search:      key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "search")),
		ContextMenu: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "menu")),
		NewFile:     key.NewBinding(key.WithKeys("ctrl+n"), key.WithHelp("ctrl+n", "new file")),
		NewFolder:   key.NewBinding(key.WithKeys("ctrl+shift+n"), key.WithHelp("ctrl+shift+n", "new folder")),
		Rename:      key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "rename")),
		Delete:      key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "delete")),
		ToggleView:  key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "toggle view")),
		CopyCurl:    key.NewBinding(key.WithKeys("ctrl+shift+c"), key.WithHelp("ctrl+shift+c", "copy curl")),
	}
}
