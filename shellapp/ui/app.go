package ui

import (
	tea "github.com/charmbracelet/bubbletea"
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

// View scans the frame for zone markers (registering click targets) and
// strips them before the renderer sees the output.
func (a *App) View() string {
	return Zones.Scan(a.screen.View())
}

func (a *App) Cleanup() {
	// Flush any pending file writes before tearing down the terminal.
	if wq := a.screen.WriteQueue; wq != nil {
		wq.FlushNow()
	}
	a.screen.Term.Close()
	Zones.Close()
}
