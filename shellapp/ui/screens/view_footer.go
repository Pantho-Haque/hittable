package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/ui/components/requesteditor"
	"github.com/hittable/shellapp/ui/theme"
)

func (m *MainScreen) renderFooter() string {
	left := m.footerText()
	if right := m.extFooter(); right != "" {
		avail := m.Width - 2
		rw := lipgloss.Width(right)
		if rw > avail/2 {
			right = ansi.Truncate(right, avail/2, "…")
			rw = lipgloss.Width(right)
		}
		left = ansi.Truncate(left, max(avail-rw-2, 0), "…")
		if pad := avail - lipgloss.Width(left) - rw; pad > 0 {
			left += strings.Repeat(" ", pad)
		}
		left += right
	}
	return theme.FooterStyle.Width(m.Width).Render(ansi.Truncate(left, m.Width-2, "…"))
}

func (m *MainScreen) footerText() string {
	if m.Sending {
		return m.Spinner.View() + " Sending… · esc cancels"
	}
	if m.StatusBar != "" {
		return m.StatusBar
	}
	if m.Palette.Open && m.attaching {
		return "pick a file to attach · ↑↓ move · ⏎ attach · alt+a type a path instead · esc cancel"
	}
	if m.Palette.ItemsOpen() {
		return "type to filter · ↑↓ move · ⏎ choose · esc cancel"
	}
	if m.Palette.Open {
		return "type to search · ↑↓ move · ⏎ open · tab find file ⇄ live grep · esc close"
	}
	if m.PathPrompt.Open {
		return "type a path (relative to this .hit, absolute, or ~/) · ⇥ complete · ⏎ attach · ctrl+w up a folder · esc cancel"
	}
	if m.ExtOpen {
		if m.ExtPanel.FocusDetail {
			return "extension · jk/↑↓ scroll · 1/2/3 or ←/→ tabs · i install · u uninstall · e enable/disable · U update · tab list · esc back"
		}
		return "extensions · type to search (@installed) · ↑↓ select · ⏎ install / open · tab details · esc clear / close"
	}
	if m.GitOpen {
		if m.Git.Resolving {
			return "merge conflict · c current · i incoming · b both · n/p next/prev · a mark resolved · o open in editor · esc back"
		}
		if m.Git.ShowOutput {
			return "git output · jk/wheel scroll · shift+↑↓ select · ctrl+c copy · y copy all · ! or esc back"
		}
		return "git · 1-5 sections · tab list⇄diff · jk move · ⏎ action · y sync/push · ! last output · esc close · ctrl+b explorer"
	}
	if m.TermFocused {
		return "terminal · keys go to the shell · drag selects, ctrl+c copies · ctrl+j hide · ctrl+b explorer"
	}
	var hints []string

	if m.ExplorerFocused {
		hints = []string{
			"↑↓/jk move",
			"⏎ open/expand",
			"x menu",
			"ctrl+n new",
			"ctrl+e rename",
			"ctrl+d delete",
			"r refresh",
			"/ find file",
			"g git",
			"ctrl+b main pane",
			"? help",
		}
	} else if m.ActiveFile != "" {
		doc := m.Store.Get(m.ActiveFile)
		if doc != nil && doc.Kind == document.KindHit {
			if m.ViewMode == ViewRunner {
				switch m.Focus {
				case FocusURLBar:
					hints = []string{"⏎ method", "ctrl+←/→ cycle method", "ctrl+r send", "tab next", "ctrl+t text mode", "ctrl+y copy curl", "esc close"}
				case FocusMethodDropdown:
					hints = []string{"←/→ choose", "⏎ select", "esc cancel"}
				case FocusTabBar:
					hints = []string{"←/→ or 1/2/3 tab", "⏎/↓ edit", "ctrl+r send", "tab next"}
				case FocusBody:
					hints = []string{"type to edit", "alt+1/2/3 tab", "ctrl+l format json", "ctrl+r send", "tab next"}
					if m.ActiveTab == requesteditor.TabBody {
						hints = []string{"alt+t body type", "alt+a attach file", "alt+1/2/3 tab", "ctrl+l format json", "ctrl+r send", "tab next"}
					}
				case FocusResponse:
					hints = []string{"jk/↑↓ scroll", "g/G top/bottom", "h headers", "ctrl+f search", "ctrl+y copy body", "tab next"}
				default:
					hints = []string{"ctrl+b explorer", "tab switch", "ctrl+r send", "ctrl+t text mode", "esc close"}
				}
			} else {
				hints = []string{
					"ctrl+t runner mode",
					"ctrl+z/ctrl+y undo/redo",
					"ctrl+f find",
					"ctrl+g go to line",
					"shift+tab explorer",
					"esc close",
				}
			}
		} else if m.isMarkdown() {
			switch m.MdMode {
			case MdPreview:
				hints = []string{"jk/↑↓ scroll", "g/G top/bottom", "ctrl+t text/split", "esc close"}
			case MdSplit:
				hints = []string{"type to edit · preview follows", "tab ⇄ preview", "ctrl+t text mode", "ctrl+z/ctrl+y undo/redo", "esc close"}
			default:
				hints = []string{"ctrl+t preview", "ctrl+z/ctrl+y undo/redo", "ctrl+f find", "ctrl+g go to line", "shift+tab explorer", "esc close"}
			}
		} else {
			hints = []string{
				"ctrl+z/ctrl+y undo/redo",
				"ctrl+f find",
				"ctrl+g go to line",
				"ctrl+←/→ word",
				"shift+tab explorer",
				"esc close",
			}
		}
	} else {
		hints = []string{
			"ctrl+b explorer",
			"↑↓ navigate",
			"⏎ open",
		}
	}

	return strings.Join(hints, " · ")
}
