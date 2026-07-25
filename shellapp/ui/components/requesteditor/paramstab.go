package requesteditor

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hittable/shellapp/ui/theme"
)

type ParamsTab struct {
	TextArea   textarea.Model
	Width      int
	Focused    bool
	InvalidJSON bool
}

func NewParamsTab() *ParamsTab {
	ta := textarea.New()
	ta.Placeholder = "{}"
	ta.CharLimit = 0
	ta.ShowLineNumbers = false
	ta.FocusedStyle.Base = lipgloss.NewStyle().Foreground(theme.TextColor)
	return &ParamsTab{
		TextArea: ta,
	}
}

func (p *ParamsTab) SetSize(w, h int) {
	p.Width = w
	p.TextArea.SetWidth(w - 2)
	p.TextArea.SetHeight(h - 2)
}

func (p *ParamsTab) SetContent(params map[string]string) {
	data, err := json.MarshalIndent(params, "", "  ")
	if err != nil {
		p.TextArea.SetValue("{}")
		return
	}
	p.TextArea.SetValue(string(data))
	p.InvalidJSON = false
}

func (p *ParamsTab) GetContent() map[string]string {
	raw := p.TextArea.Value()
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		p.InvalidJSON = true
		return nil
	}
	if result == nil {
		result = map[string]string{}
	}
	p.InvalidJSON = false
	return result
}

func (p *ParamsTab) Focus() {
	p.Focused = true
	p.TextArea.Focus()
}

func (p *ParamsTab) Blur() {
	p.Focused = false
	p.TextArea.Blur()
}

func (p *ParamsTab) Update(msg tea.Msg) tea.Cmd {
	if !p.Focused {
		return nil
	}
	var cmd tea.Cmd
	p.TextArea, cmd = p.TextArea.Update(msg)
	if cmd != nil {
		return cmd
	}
	raw := p.TextArea.Value()
	if strings.TrimSpace(raw) == "" {
		p.InvalidJSON = false
		return nil
	}
	var m map[string]string
	p.InvalidJSON = json.Unmarshal([]byte(raw), &m) != nil
	return nil
}

func (p *ParamsTab) View() string {
	v := p.TextArea.View()
	if p.InvalidJSON {
		return v + "\n" + theme.StatusErrorStyle.Render(" ⚠ invalid JSON")
	}
	return v
}
