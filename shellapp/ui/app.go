package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/hittable/shellapp/ui/screens"
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

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return a.screen.Update(msg)
}

func (a *App) View() string {
	return Zones.Scan(a.screen.View())
}

func (a *App) Cleanup() {
	Zones.Close()
}
