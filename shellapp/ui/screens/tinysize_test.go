package screens

import (
	"fmt"
	"path/filepath"
	"runtime/debug"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/ui/components/palette"
)

// A user can shrink the window to anything — a split pane, a zoomed-in
// terminal, a tmux status pane — and the app must keep running. Every view
// is laid out, rendered and poked with mouse and keys at sizes far below the
// smallest supported layout; none of it may panic.
func TestTinyTerminalSizesNeverPanic(t *testing.T) {
	views := map[string]func(m *MainScreen, hd string){
		"welcome": func(m *MainScreen, hd string) {},
		"runner":  func(m *MainScreen, hd string) { m.openFileRaw(filepath.Join(hd, "a.hit")) },
		"hit-text": func(m *MainScreen, hd string) {
			m.openFileRaw(filepath.Join(hd, "a.hit"))
			m.toggleViewMode()
		},
		"markdown-text": func(m *MainScreen, hd string) { m.openFileRaw(filepath.Join(hd, "notes.md")) },
		"markdown-preview": func(m *MainScreen, hd string) {
			m.openFileRaw(filepath.Join(hd, "notes.md"))
			m.setMdMode(MdPreview)
		},
		"markdown-split": func(m *MainScreen, hd string) {
			m.openFileRaw(filepath.Join(hd, "notes.md"))
			m.setMdMode(MdSplit)
		},
		"terminal": func(m *MainScreen, hd string) {
			m.openFileRaw(filepath.Join(hd, "a.hit"))
			m.Term.Open = true // the panel, without spawning a shell
			m.focusTerminal()
		},
		"git":     func(m *MainScreen, hd string) { m.toggleGit() },
		"palette": func(m *MainScreen, hd string) { m.openPalette(palette.ModeFiles) },
		"grep":    func(m *MainScreen, hd string) { m.openPalette(palette.ModeGrep) },
		"help":    func(m *MainScreen, hd string) { m.toggleHelp() },
		"hidden-sidebar": func(m *MainScreen, hd string) {
			m.openFileRaw(filepath.Join(hd, "a.hit"))
			m.toggleExplorer()
		},
		"dropdown": func(m *MainScreen, hd string) {
			m.openFileRaw(filepath.Join(hd, "a.hit"))
			m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // URL bar focused: opens the method picker
		},
		"response-search": func(m *MainScreen, hd string) {
			m.openFileRaw(filepath.Join(hd, "a.hit"))
			m.Response.SetResponse(200, "OK", 1, 12, true, `{"a":[1,2,3]}`)
			m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
		},
		"explorer-menu": func(m *MainScreen, hd string) {
			m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		},
	}
	sizes := [][2]int{{0, 0}, {1, 1}, {5, 3}, {10, 5}, {20, 8}, {300, 1}, {1, 300}, {2, 50}, {21, 12}, {35, 17}, {400, 200}}

	for name, setup := range views {
		for _, sz := range sizes {
			t.Run(fmt.Sprintf("%s/%dx%d", name, sz[0], sz[1]), func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic: %v\n%s", r, debug.Stack())
					}
				}()
				t.Setenv("SHELL", "/bin/sh") // keys in a focused panel may start a shell
				m, hd := newTestScreen(t)    // laid out at a sane size first
				t.Cleanup(m.Term.Close)
				setup(m, hd)
				m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
				_ = m.OverlaySelection(m.Zones.Scan(m.View()))
				// Mouse and keys at the edges of the tiny frame.
				for _, p := range [][2]int{{0, 0}, {sz[0] / 2, sz[1] / 2}, {sz[0], sz[1]}, {sz[0] + 5, sz[1] + 5}} {
					m.Update(tea.MouseMsg{Type: tea.MouseMotion, Action: tea.MouseActionMotion, X: p[0], Y: p[1]})
					m.Update(press(p[0], p[1]))
					m.Update(moveTo(p[0]+1, p[1]+1))
					m.Update(release(p[0]+1, p[1]+1))
					m.Update(tea.MouseMsg{Type: tea.MouseWheelDown, Button: tea.MouseButtonWheelDown, X: p[0], Y: p[1]})
				}
				for _, k := range []tea.KeyType{tea.KeyDown, tea.KeyUp, tea.KeyPgDown, tea.KeyEnd, tea.KeyHome} {
					m.Update(tea.KeyMsg{Type: k})
				}
				_ = m.View()
				// Growing back must restore a full-size frame.
				m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
				_ = m.View()
			})
		}
	}
}
