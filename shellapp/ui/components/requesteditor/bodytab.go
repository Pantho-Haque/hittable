package requesteditor

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hittable/shellapp/ui/theme"
)

type BodyTab struct {
	TextArea textarea.Model
	Width    int
	Focused  bool
}

func NewBodyTab() *BodyTab {
	ta := textarea.New()
	ta.Placeholder = `{"key": "value"}`
	ta.CharLimit = 0
	ta.ShowLineNumbers = false
	ta.FocusedStyle.Base = lipgloss.NewStyle().Foreground(theme.TextColor)
	return &BodyTab{
		TextArea: ta,
	}
}

func (b *BodyTab) SetSize(w, h int) {
	b.Width = w
	b.TextArea.SetWidth(w - 2)
	b.TextArea.SetHeight(h - 2)
}

func (b *BodyTab) SetContent(body string) {
	b.TextArea.SetValue(body)
}

func (b *BodyTab) GetContent() string {
	return b.TextArea.Value()
}

func (b *BodyTab) Focus() {
	b.Focused = true
	b.TextArea.Focus()
}

func (b *BodyTab) Blur() {
	b.Focused = false
	b.TextArea.Blur()
}

func (b *BodyTab) Update(msg tea.Msg) tea.Cmd {
	if !b.Focused {
		return nil
	}
	var cmd tea.Cmd
	b.TextArea, cmd = b.TextArea.Update(msg)
	return cmd
}

func (b *BodyTab) View() string {
	return b.TextArea.View()
}
