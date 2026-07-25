package screens

import (
	"strings"

	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/ui/theme"
)

func (m *MainScreen) renderFooter() string {
	var hints []string

	if m.ExplorerFocused {
		hints = []string{
			"ctrl+b main",
			"↑↓ navigate",
			"⏎ open/expand",
			"x menu",
			"drag │ resize",
		}
	} else if m.ActiveFile != "" {
		doc := m.Store.Get(m.ActiveFile)
		if doc != nil && doc.Kind == document.KindHit {
			if m.ViewMode == ViewRunner {
				hints = []string{
					"ctrl+b explorer",
					"tab switch",
					"ctrl+r send",
					"ctrl+s save",
					"ctrl+t text mode",
					"↑↓ scroll",
					"esc close",
				}
			} else {
				hints = []string{
					"ctrl+b explorer",
					"ctrl+s save",
					"ctrl+t runner mode",
					"ctrl+f search",
					"esc close",
				}
			}
		} else {
			hints = []string{
				"ctrl+b explorer",
				"ctrl+s save",
				"esc close",
				"ctrl+c quit",
			}
		}
	} else {
		hints = []string{
			"ctrl+b explorer",
			"↑↓ navigate",
			"⏎ open",
		}
	}

	footer := strings.Join(hints, " · ")
	return theme.FooterStyle.
		Width(m.Width).
		Render(footer)
}
