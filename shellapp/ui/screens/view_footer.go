package screens

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/ui/theme"
)

func (m *MainScreen) renderFooter() string {
	return theme.FooterStyle.Width(m.Width).Render(ansi.Truncate(m.footerText(), m.Width-2, "…"))
}

func (m *MainScreen) footerText() string {
	if m.Sending {
		return m.Spinner.View() + " Sending…"
	}
	if m.StatusBar != "" {
		return m.StatusBar
	}
	if m.Palette.Open {
		return "type to search · ↑↓ move · ⏎ open · tab find file ⇄ live grep · esc close"
	}
	if m.GitOpen {
		if m.Git.Resolving {
			return "merge conflict · c current · i incoming · b both · n/p next/prev · a mark resolved · o open in editor · esc back"
		}
		return "git · 1-5 sections · jk move · ⏎ action · y sync/push · esc close · ctrl+b explorer"
	}
	if m.TermFocused {
		return "terminal · keys go to the shell · ctrl+j hide · ctrl+b explorer · click editor to leave"
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
