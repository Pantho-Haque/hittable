package requesteditor

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hittable/shellapp/ui/theme"
)

type HeadersTab struct {
	TextArea    textarea.Model
	Width       int
	Focused     bool
	InvalidJSON bool
}

func NewHeadersTab() *HeadersTab {
	ta := textarea.New()
	ta.Placeholder = "{}"
	ta.CharLimit = 0
	ta.ShowLineNumbers = false
	ta.FocusedStyle.Base = lipgloss.NewStyle().Foreground(theme.TextColor)
	return &HeadersTab{
		TextArea: ta,
	}
}

func (h *HeadersTab) SetSize(w, hh int) {
	h.Width = w
	h.TextArea.SetWidth(w - 2)
	h.TextArea.SetHeight(hh - 2)
}

func (h *HeadersTab) SetContent(headers map[string]string) {
	data, err := json.MarshalIndent(headers, "", "  ")
	if err != nil {
		h.TextArea.SetValue("{}")
		return
	}
	h.TextArea.SetValue(string(data))
	h.InvalidJSON = false
}

func (h *HeadersTab) GetContent() map[string]string {
	raw := h.TextArea.Value()
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		h.InvalidJSON = true
		return nil
	}
	if result == nil {
		result = map[string]string{}
	}
	h.InvalidJSON = false
	return result
}

func (h *HeadersTab) Focus() {
	h.Focused = true
	h.TextArea.Focus()
}

func (h *HeadersTab) Blur() {
	h.Focused = false
	h.TextArea.Blur()
}

func (h *HeadersTab) Update(msg tea.Msg) tea.Cmd {
	if !h.Focused {
		return nil
	}
	var cmd tea.Cmd
	h.TextArea, cmd = h.TextArea.Update(msg)
	if cmd != nil {
		return cmd
	}
	raw := h.TextArea.Value()
	if strings.TrimSpace(raw) == "" {
		h.InvalidJSON = false
		return nil
	}
	var m map[string]string
	h.InvalidJSON = json.Unmarshal([]byte(raw), &m) != nil
	return nil
}

func (h *HeadersTab) View() string {
	v := h.TextArea.View()
	if h.InvalidJSON {
		return v + "\n" + theme.StatusErrorStyle.Render(" ⚠ invalid JSON")
	}
	return v
}
