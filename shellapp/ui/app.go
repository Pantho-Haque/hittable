package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/hittable/shellapp/ui/screens"
	zone "github.com/lrstanley/bubblezone"
)

var Zones = zone.New()

type App struct {
	screen *screens.MainScreen
}

func NewApp(rootDir string) *App {
	return &App{
		screen: screens.NewMainScreen(rootDir, Zones),
	}
}

func (a *App) Init() tea.Cmd {
	return a.screen.Init()
}

// Update must return the App itself. Returning the inner screen would make
// bubbletea call the screen's View directly from then on, bypassing the
// bubblezone Scan below — no zone would ever register and every main-pane
// click would be ignored.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := a.screen.Update(msg)
	return a, cmd
}

// View scans the frame for zone markers (registering click targets), strips
// them, and clips the result to the terminal.
//
// The clip is not cosmetic. A frame one row too tall makes the terminal
// scroll, which pushes the top bar off the screen and leaves every mouse
// coordinate out of step with the layout by the number of scrolled rows —
// clicks and hovers then land on the wrong row. Clipping here keeps that
// contained to the pane that misbehaved. It runs after Scan so the zone
// coordinates match what is actually on screen.
func (a *App) View() string {
	return clipToTerminal(Zones.Scan(a.screen.View()), a.screen.Width, a.screen.Height)
}

func clipToTerminal(s string, w, h int) string {
	if w <= 0 || h <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, l := range lines {
		if lipgloss.Width(l) > w {
			lines[i] = ansi.Truncate(l, w, "")
		}
	}
	return strings.Join(lines, "\n")
}

func (a *App) Cleanup() {
	// Flush any pending file writes before tearing down the terminal.
	if wq := a.screen.WriteQueue; wq != nil {
		wq.FlushNow()
	}
	a.screen.Term.Close()
	Zones.Close()
}
