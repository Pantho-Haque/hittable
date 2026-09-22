package requesteditor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

// URLBar is the method badge + URL input. The method picker renders inline
// in place of the URL line (no layout shift) while DropdownOpen.
type URLBar struct {
	// Hover is the zone id the mouse is over, fed by the screen each frame.
	Hover string

	URLInput textinput.Model
	Width    int
	Focused  bool
	Method   string

	DropdownOpen bool
	DropdownIdx  int
}

func NewURLBar() *URLBar {
	url := textinput.New()
	url.Placeholder = "https://api.example.com/endpoint  (<<KEY>> resolves from env.json)"
	url.CharLimit = 2000
	url.Prompt = ""
	return &URLBar{URLInput: url, Method: "GET"}
}

// badgeWidth is the rendered width of the method badge plus its gap; the URL
// input starts at this column inside the bar.
const badgeWidth = 10
const sendLabel = " ▶ Send "

func (u *URLBar) SetSize(w int) {
	u.Width = w
	u.URLInput.Width = w - badgeWidth - len(sendLabel) - 3
	if u.URLInput.Width < 10 {
		u.URLInput.Width = 10
	}
}

// ClickAt moves the input cursor to the clicked column (relative to the
// bar's inner left edge).
func (u *URLBar) ClickAt(x int) {
	p := x - badgeWidth
	if p < 0 {
		p = 0
	}
	u.URLInput.SetCursor(p) // clamps to the value length
}

func (u *URLBar) SetContent(method, url string) {
	if method == "" {
		method = "GET"
	}
	u.Method = strings.ToUpper(method)
	u.URLInput.SetValue(url)
	u.URLInput.CursorEnd()
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
	if u.DropdownOpen {
		u.CloseDropdown()
		return
	}
	u.DropdownOpen = true
	u.DropdownIdx = 0
	for i, m := range Methods {
		if m == u.Method {
			u.DropdownIdx = i
		}
	}
}

func (u *URLBar) CloseDropdown() { u.DropdownOpen = false }

func (u *URLBar) SelectMethod(idx int) {
	if idx >= 0 && idx < len(Methods) {
		u.Method = Methods[idx]
	}
	u.DropdownOpen = false
}

// CycleMethod steps the method without opening the picker.
func (u *URLBar) CycleMethod(dir int) {
	for i, m := range Methods {
		if m == u.Method {
			u.Method = Methods[(i+dir+len(Methods))%len(Methods)]
			return
		}
	}
	u.Method = Methods[0]
}

func (u *URLBar) HandleDropdownKey(msg tea.KeyMsg) bool {
	if !u.DropdownOpen {
		return false
	}
	switch msg.String() {
	case "left", "up", "k", "h", "shift+tab":
		u.DropdownIdx = (u.DropdownIdx + len(Methods) - 1) % len(Methods)
	case "right", "down", "j", "l", "tab":
		u.DropdownIdx = (u.DropdownIdx + 1) % len(Methods)
	case "enter", " ":
		u.SelectMethod(u.DropdownIdx)
	case "esc":
		u.CloseDropdown()
	default:
		// Typing a method's first letter jumps to it.
		s := strings.ToUpper(msg.String())
		for i, m := range Methods {
			if strings.HasPrefix(m, s) && i != u.DropdownIdx {
				u.DropdownIdx = i
				break
			}
		}
	}
	return true
}

func (u *URLBar) Update(msg tea.Msg) tea.Cmd {
	if u.DropdownOpen || !u.Focused {
		return nil
	}
	var cmd tea.Cmd
	u.URLInput, cmd = u.URLInput.Update(msg)
	return cmd
}

func (u *URLBar) View(z *zone.Manager) string {
	badge := z.Mark("method_badge", theme.Hoverable(u.Hover == "method_badge",
		theme.MethodStyle.Foreground(theme.MethodColor(u.Method))).
		Render(fmt.Sprintf(" %-7s▾", u.Method)))

	var bar string
	if u.DropdownOpen {
		// The picker replaces the whole bar so it never changes the row count.
		items := []string{theme.MutedStyle.Render("▾")}
		for i, m := range Methods {
			st := theme.MutedStyle
			if i == u.DropdownIdx {
				st = theme.CursorFocusedStyle.Foreground(theme.MethodColor(m))
			}
			items = append(items, z.Mark("method_"+m, theme.Hoverable(u.Hover == "method_"+m, st).Render(m)))
		}
		bar = ansi.Truncate(strings.Join(items, " "), u.Width, "…")
	} else {
		send := z.Mark("send_btn", theme.Hoverable(u.Hover == "send_btn", theme.SendButtonStyle).Render(sendLabel))
		url := theme.URLStyle.Render(u.URLInput.View())
		gap := u.Width - badgeWidth - lipgloss.Width(url) - lipgloss.Width(send)
		if gap < 1 {
			gap = 1
		}
		bar = lipgloss.JoinHorizontal(lipgloss.Top, badge, " ", url, strings.Repeat(" ", gap), send)
	}

	style := theme.UnfocusedBorderStyle
	if u.Focused || u.DropdownOpen {
		style = theme.FocusedBorderStyle
	}
	return style.Width(u.Width).Render(bar)
}
