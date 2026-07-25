// Package texteditor implements a Vim-style text editor using vimtea.
package texteditor

import (
	"fmt"
	"sync"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hittable/shellapp/ui/theme"
	"github.com/kujtimiihoxha/vimtea"
)

type TextEditor struct {
	Editor    vimtea.Editor
	Width     int
	Height    int
	Focused   bool
	InInsert  bool
	InVisual  bool
	OnChanged func(content string)

	editors  map[string]vimtea.Editor
	editorsMu sync.RWMutex
}

func New() *TextEditor {
	editor := vimtea.NewEditor(
		vimtea.WithEnableStatusBar(true),
		vimtea.WithRelativeNumbers(false),
		vimtea.WithEnableModeCommand(false),
		vimtea.WithTextStyle(lipgloss.NewStyle().Foreground(theme.TextColor)),
		vimtea.WithLineNumberStyle(lipgloss.NewStyle().Foreground(theme.MutedColor)),
		vimtea.WithCurrentLineNumberStyle(lipgloss.NewStyle().Foreground(theme.PrimaryColor)),
		vimtea.WithStatusStyle(lipgloss.NewStyle().Foreground(theme.MutedColor).Background(theme.SurfaceColor)),
		vimtea.WithCursorStyle(lipgloss.NewStyle().Background(theme.PrimaryColor).Foreground(theme.BgColor)),
	)
	return &TextEditor{
		Editor:  editor,
		editors: make(map[string]vimtea.Editor),
	}
}

func (t *TextEditor) SetSize(w, h int) {
	t.Width = w
	t.Height = h
	model, _ := t.Editor.SetSize(w-2, h-2)
	if e, ok := model.(vimtea.Editor); ok {
		t.Editor = e
	}
}

func (t *TextEditor) getOrCreateEditor(path, content string) vimtea.Editor {
	t.editorsMu.RLock()
	if ed, ok := t.editors[path]; ok {
		t.editorsMu.RUnlock()
		return ed
	}
	t.editorsMu.RUnlock()

	ed := vimtea.NewEditor(
		vimtea.WithContent(content),
		vimtea.WithEnableStatusBar(true),
		vimtea.WithRelativeNumbers(false),
		vimtea.WithEnableModeCommand(false),
		vimtea.WithTextStyle(lipgloss.NewStyle().Foreground(theme.TextColor)),
		vimtea.WithLineNumberStyle(lipgloss.NewStyle().Foreground(theme.MutedColor)),
		vimtea.WithCurrentLineNumberStyle(lipgloss.NewStyle().Foreground(theme.PrimaryColor)),
		vimtea.WithStatusStyle(lipgloss.NewStyle().Foreground(theme.MutedColor).Background(theme.SurfaceColor)),
		vimtea.WithCursorStyle(lipgloss.NewStyle().Background(theme.PrimaryColor).Foreground(theme.BgColor)),
	)

	t.editorsMu.Lock()
	t.editors[path] = ed
	t.editorsMu.Unlock()
	return ed
}

func (t *TextEditor) SetContent(path, content string) {
	t.Editor = t.getOrCreateEditor(path, content)
	if t.Width > 0 && t.Height > 0 {
		model, _ := t.Editor.SetSize(t.Width-2, t.Height-2)
		if e, ok := model.(vimtea.Editor); ok {
			t.Editor = e
			t.editorsMu.Lock()
			t.editors[path] = e
			t.editorsMu.Unlock()
		}
	}
}

func (t *TextEditor) ReplaceContent(content string) {
	buf := t.Editor.GetBuffer()
	if buf == nil {
		return
	}
	lines := buf.Lines()
	totalLines := len(lines)
	if totalLines > 0 {
		lastLineLen := len([]rune(lines[totalLines-1]))
		buf.DeleteAt(0, 0, totalLines-1, lastLineLen)
	}
	if len(content) > 0 {
		buf.InsertAt(0, 0, content)
	}
}

func (t *TextEditor) GetContent() string {
	return t.Editor.GetBuffer().Text()
}

func (t *TextEditor) GetCursorRow() int {
	buf := t.Editor.GetBuffer()
	if buf == nil {
		return 0
	}
	lines := buf.Lines()
	if len(lines) == 0 {
		return 0
	}
	return len(lines) - 1
}

func (t *TextEditor) GetTotalLines() int {
	buf := t.Editor.GetBuffer()
	if buf == nil {
		return 0
	}
	return len(buf.Lines())
}

func (t *TextEditor) Focus() {
	t.Focused = true
	t.Editor.SetMode(vimtea.ModeInsert)
}

func (t *TextEditor) Blur() {
	t.Focused = false
	t.Editor.SetMode(vimtea.ModeNormal)
}

func (t *TextEditor) RemoveEditor(path string) {
	t.editorsMu.Lock()
	delete(t.editors, path)
	t.editorsMu.Unlock()
}

func (t *TextEditor) Update(msg tea.Msg) tea.Cmd {
	if !t.Focused {
		return nil
	}

	if modeMsg, ok := msg.(vimtea.EditorModeMsg); ok {
		t.InInsert = modeMsg.Mode == vimtea.ModeInsert
		t.InVisual = modeMsg.Mode == vimtea.ModeVisual
	}

	if !t.InInsert {
		if km, ok := msg.(tea.KeyMsg); ok {
			if key.Matches(km, key.NewBinding(key.WithKeys("ctrl+b"))) {
				return nil
			}
			if key.Matches(km, key.NewBinding(key.WithKeys("ctrl+t"))) {
				return nil
			}
			if key.Matches(km, key.NewBinding(key.WithKeys("ctrl+s"))) {
				return nil
			}
			if key.Matches(km, key.NewBinding(key.WithKeys("ctrl+r"))) {
				return nil
			}
			if key.Matches(km, key.NewBinding(key.WithKeys("ctrl+f"))) {
				return nil
			}
			if key.Matches(km, key.NewBinding(key.WithKeys("ctrl+enter"))) {
				return nil
			}
		}
	}

	model, cmd := t.Editor.Update(msg)
	if e, ok := model.(vimtea.Editor); ok {
		t.Editor = e
	}

	if t.OnChanged != nil {
		t.OnChanged(t.GetContent())
	}

	return cmd
}

func (t *TextEditor) View() string {
	border := theme.UnfocusedBorderStyle
	if t.Focused {
		border = theme.FocusedBorderStyle
	}
	editorView := t.Editor.View()
	scrollInfo := fmt.Sprintf(" Ln %d/%d", t.GetCursorRow()+1, t.GetTotalLines())
	scrollIndicator := theme.MutedStyle.Render(scrollInfo)
	return border.Width(t.Width).Height(t.Height).Render(editorView + scrollIndicator)
}
