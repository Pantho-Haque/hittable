package ui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/hittable/shellapp/ui/screens"
	zone "github.com/lrstanley/bubblezone"
)

var Zones = zone.New()

type App struct {
	screen *screens.MainScreen

	// Frame coalescing. Bubble Tea builds a frame after every message, and a
	// trackpad sends wheel and motion events faster than a frame is built on
	// a big screen: they queued up, and the view kept scrolling after the
	// hand stopped. Those events are still applied as they arrive (cheap),
	// but within frameGap of the last frame View hands back that frame and
	// one tick draws the latest state.
	last    string
	lastAt  time.Time
	hold    bool // View may return last: a frame tick is on its way
	ticking bool
}

// frameGap is the shortest time between two frames for wheel and motion (~60 fps).
const frameGap = 16 * time.Millisecond

// frameMsg draws the state a held frame skipped.
type frameMsg struct{}

// coalesced reports the events that only move things: a wheel tick, or the
// pointer moving (hover, or a drag in progress).
func coalesced(msg tea.Msg) bool {
	m, ok := msg.(tea.MouseMsg)
	if !ok {
		return false
	}
	switch m.Type {
	case tea.MouseWheelUp, tea.MouseWheelDown, tea.MouseWheelLeft, tea.MouseWheelRight, tea.MouseMotion:
		return true
	}
	return m.Action == tea.MouseActionMotion
}

// NewFileApp opens one file on its own (`hittable <file>`).
func NewFileApp(path string) *App {
	a := NewApp(filepath.Dir(path))
	a.screen.EnterSingleFile(path)
	return a
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
	if _, ok := msg.(frameMsg); ok {
		a.ticking, a.hold = false, false
		return a, nil
	}
	_, cmd := a.screen.Update(msg)
	a.hold = false
	if coalesced(msg) && a.last != "" {
		if since := time.Since(a.lastAt); since < frameGap {
			a.hold = true
			if !a.ticking {
				a.ticking = true
				cmd = tea.Batch(cmd, tea.Tick(frameGap-since, func(time.Time) tea.Msg { return frameMsg{} }))
			}
		}
	}
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
	if a.hold {
		return a.last
	}
	frame := clipToTerminal(a.screen.OverlaySelection(Zones.Scan(a.screen.View())), a.screen.Width, a.screen.Height)
	// The mouse pointer is the terminal's, not ours: OSC 22 asks for a shape
	// (kitty, Ghostty, WezTerm, foot, xterm); others ignore it. It rides on
	// the first line, so it only reaches the terminal when the shape changes.
	a.last, a.lastAt = pointerShape(a.screen.PointerShape())+frame, time.Now()
	return a.last
}

func pointerShape(shape string) string { return "\x1b]22;" + shape + "\x1b\\" }

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
	os.Stdout.WriteString(pointerShape("default"))
	// Flush any pending file writes before tearing down the terminal.
	if wq := a.screen.WriteQueue; wq != nil {
		wq.FlushNow()
	}
	a.screen.Term.Close()
	// Stop a model server this process started. One adopted from another
	// hittable window is left running for it.
	a.screen.CloseAI()
	a.screen.CloseExtensions()
	a.screen.ExtPanel.DeleteImages(os.Stdout)
	Zones.Close()
}
