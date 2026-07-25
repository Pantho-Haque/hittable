package requesteditor

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hittable/shellapp/ui/theme"
	zone "github.com/lrstanley/bubblezone"
)

type Tab int

const (
	TabParams Tab = iota
	TabHeaders
	TabBody
)

var Methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

type URLBar struct {
	MethodInput textinput.Model
	URLInput    textinput.Model
	Width       int
	Focused     bool
	Method      string

	DropdownOpen bool
	DropdownIdx  int

	OnMethodSelected func(method string)
	OnDropdownClosed func()
}

func NewURLBar() *URLBar {
	method := textinput.New()
	method.Placeholder = "GET"
	method.CharLimit = 10
	method.Width = 10
	url := textinput.New()
	url.Placeholder = "https://api.example.com/endpoint"
	url.CharLimit = 500
	return &URLBar{
		MethodInput: method,
		URLInput:    url,
		Method:      "GET",
	}
}

func (u *URLBar) SetSize(w int) {
	u.Width = w
	u.URLInput.Width = w - 18
}

func (u *URLBar) SetContent(method, url string) {
	u.Method = method
	u.MethodInput.SetValue(method)
	u.URLInput.SetValue(url)
}

func (u *URLBar) GetContent() (string, string) {
	return u.Method, u.URLInput.Value()
}

func (u *URLBar) Focus() {
	u.Focused = true
	u.URLInput.Focus()
}

func (u *URLBar) Blur() {
	u.Focused = false
	u.URLInput.Blur()
}

func (u *URLBar) ToggleDropdown() {
	u.DropdownOpen = !u.DropdownOpen
	if u.DropdownOpen {
		for i, m := range Methods {
			if m == u.Method {
				u.DropdownIdx = i
				break
			}
		}
	} else {
		if u.OnDropdownClosed != nil {
			u.OnDropdownClosed()
		}
	}
}

func (u *URLBar) CloseDropdown() {
	if u.DropdownOpen {
		u.DropdownOpen = false
		if u.OnDropdownClosed != nil {
			u.OnDropdownClosed()
		}
	}
}

func (u *URLBar) SelectMethod(idx int) {
	if idx >= 0 && idx < len(Methods) {
		u.Method = Methods[idx]
		u.MethodInput.SetValue(Methods[idx])
		if u.OnMethodSelected != nil {
			u.OnMethodSelected(Methods[idx])
		}
	}
	u.DropdownOpen = false
	if u.OnDropdownClosed != nil {
		u.OnDropdownClosed()
	}
}

func (u *URLBar) HandleDropdownKey(msg tea.KeyMsg) bool {
	if !u.DropdownOpen {
		return false
	}
	switch msg.String() {
	case "up", "k":
		if u.DropdownIdx > 0 {
			u.DropdownIdx--
		}
		return true
	case "down", "j":
		if u.DropdownIdx < len(Methods)-1 {
			u.DropdownIdx++
		}
		return true
	case "enter", " ":
		u.SelectMethod(u.DropdownIdx)
		return true
	case "esc":
		u.CloseDropdown()
		return true
	}
	return true
}

func (u *URLBar) Update(msg tea.Msg) tea.Cmd {
	if u.DropdownOpen {
		return nil
	}
	if !u.Focused {
		return nil
	}
	var cmd tea.Cmd
	u.URLInput, cmd = u.URLInput.Update(msg)
	return cmd
}

func (u *URLBar) View(z *zone.Manager) string {
	method := z.Mark("method_badge", theme.MethodStyle.Render(fmt.Sprintf(" %-6s ▾", u.Method)))
	url := theme.URLStyle.Render(u.URLInput.View())
	bar := lipgloss.JoinHorizontal(lipgloss.Top, method, " ", url)

	var result string
	if u.Focused {
		result = theme.FocusedBorderStyle.Width(u.Width).Render(bar)
	} else {
		result = theme.UnfocusedBorderStyle.Width(u.Width).Render(bar)
	}

	if u.DropdownOpen {
		var menuLines []string
		for i, m := range Methods {
			style := theme.ContextMenuItemStyle
			if i == u.DropdownIdx {
				style = theme.ContextMenuItemHoverStyle
			}
			item := style.Render(fmt.Sprintf(" %-8s ", m))
			menuLines = append(menuLines, z.Mark(fmt.Sprintf("method_%s", m), item))
		}
		menu := lipgloss.JoinVertical(lipgloss.Left, menuLines...)
		dropdown := theme.ContextMenuStyle.Render(menu)
		result = result + "\n" + dropdown
	}

	return result
}
