package screens

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Every shortcut in the reference has to be reachable at any terminal size:
// the overlay must fit the pane, and scroll when the content outgrows it.
func TestHelpFitsAndScrolls(t *testing.T) {
	total := 0
	for _, sec := range helpSections {
		total += len(sec.rows)
	}
	if total < 60 {
		t.Fatalf("keyboard reference only lists %d shortcuts", total)
	}
	first := helpSections[0].title
	last := helpSections[len(helpSections)-1].title

	for _, size := range [][2]int{{220, 60}, {200, 50}, {160, 45}, {120, 40}, {90, 30}} {
		m := NewMainScreen(t.TempDir(), nil)
		m.SetSize(size[0], size[1])
		m.ShowHelp = true

		top := m.renderHelp()
		if got, want := lipgloss.Height(top), m.MainH; got > want {
			t.Errorf("%dx%d: help is %d rows, pane is %d", size[0], size[1], got, want)
		}
		if !strings.Contains(top, first) {
			t.Errorf("%dx%d: first section %q missing at the top", size[0], size[1], first)
		}

		// Scrolling to the bottom must bring the last section into view.
		m.HelpScroll = 1 << 20
		bottom := m.renderHelp()
		if got, want := lipgloss.Height(bottom), m.MainH; got > want {
			t.Errorf("%dx%d: scrolled help is %d rows, pane is %d", size[0], size[1], got, want)
		}
		if !strings.Contains(top, last) && !strings.Contains(bottom, last) {
			t.Errorf("%dx%d: last section %q unreachable even scrolled", size[0], size[1], last)
		}
	}
}

// The wrap toggle has to be discoverable on macOS, where Option+z arrives as Ω.
func TestHelpDocumentsMacWrapKey(t *testing.T) {
	for _, sec := range helpSections {
		for _, r := range sec.rows {
			if strings.Contains(r.desc, "wrap") && strings.Contains(r.key, "⌥z") {
				return
			}
		}
	}
	t.Fatal("no help row shows the macOS wrap shortcut")
}
