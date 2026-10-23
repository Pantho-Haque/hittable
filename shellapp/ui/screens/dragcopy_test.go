package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// copied reports a successful copy, or the clipboard being absent (CI has
// none) — either way the selection, not a quit or an interrupt, handled it.
func copied(status string) bool {
	return strings.HasPrefix(status, "copied ") || strings.EqualFold(status, "clipboard unavailable")
}

// cellOf finds text in the last drawn frame, in screen cells.
func cellOf(t *testing.T, m *MainScreen, s string) (int, int) {
	t.Helper()
	for y, l := range m.lastFrame {
		plain := ansi.Strip(l)
		if i := strings.Index(plain, s); i >= 0 {
			return ansi.StringWidth(plain[:i]), y
		}
	}
	t.Fatalf("%q not on screen", s)
	return 0, 0
}

func draw(m *MainScreen) { m.OverlaySelection(m.Zones.Scan(m.View())) }

// Releasing a drag never copies: the block stays highlighted, the footer
// says how to copy, ctrl+c copies instead of quitting, and only then does a
// plain ctrl+c quit again.
func TestDragReleaseKeepsSelectionUntilCtrlC(t *testing.T) {
	m, hd := newTestScreen(t)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	m.Response.SetResponse(200, "OK", 3, 20, true, `{"alpha":"beta"}`)
	draw(m)
	time.Sleep(40 * time.Millisecond)
	draw(m)
	x, y := cellOf(t, m, `"alpha"`)
	// The response pane has no selection of its own, so this is a frame drag.
	m.Update(press(x, y))
	m.Update(moveTo(x+6, y))
	m.Update(release(x+6, y))
	if !m.frameSel.active {
		t.Fatal("release dropped the selection")
	}
	if m.StatusBar != "ctrl+c copies the selection" {
		t.Errorf("footer after release = %q", m.StatusBar)
	}
	if got := m.frameSelText(); got != `"alpha"` {
		t.Errorf("selected %q", got)
	}
	out := m.OverlaySelection(m.Zones.Scan(m.View()))
	if !strings.Contains(out, "\x1b[") || !strings.Contains(ansi.Strip(out), `"alpha"`) {
		t.Error("selection is not highlighted in the frame")
	}

	_, cmd := ctrl(m, tea.KeyCtrlC)
	if isQuit(cmd) {
		t.Fatal("ctrl+c with a selection must copy, not quit")
	}
	if !copied(m.StatusBar) {
		t.Errorf("status after ctrl+c = %q", m.StatusBar)
	}
	if !m.frameSel.active {
		t.Error("the highlight should stay after copying")
	}

	// A plain click drops it; ctrl+c quits again.
	m.Update(press(x, y))
	m.Update(release(x, y))
	if m.frameSel.active {
		t.Error("a click should drop the selection")
	}
	if _, cmd := ctrl(m, tea.KeyCtrlC); !isQuit(cmd) {
		t.Error("ctrl+c without a selection quits")
	}
}

// Any key other than ctrl+c lets the highlight go and still does its job.
func TestKeyDropsFrameSelection(t *testing.T) {
	m, hd := newTestScreen(t)
	m.openFileRaw(filepath.Join(hd, "notes.md"))
	draw(m)
	x, y := cellOf(t, m, "hittable/")
	m.Update(press(x, y))
	m.Update(moveTo(x+4, y))
	m.Update(release(x+4, y))
	if !m.frameSel.active {
		t.Fatal("a drag over the breadcrumb should select the frame")
	}
	before := m.ExplorerFocused
	ctrl(m, tea.KeyCtrlB) // explorer <-> main toggle still happens
	if m.frameSel.active {
		t.Error("key should drop the selection")
	}
	if m.ExplorerFocused == before {
		t.Error("the key was swallowed instead of acting")
	}
}

// The help overlay is selectable text too.
func TestHelpTextSelects(t *testing.T) {
	m, _ := newTestScreen(t)
	m.toggleHelp()
	draw(m)
	x, y := cellOf(t, m, "ctrl+r")
	m.Update(press(x, y))
	m.Update(moveTo(x+5, y))
	m.Update(release(x+5, y))
	if got := m.frameSelText(); got != "ctrl+r" {
		t.Fatalf("selected %q from help", got)
	}
	if !m.ShowHelp {
		t.Fatal("selecting in help should not close it")
	}
	// A plain click still dismisses it.
	m.Update(press(x, y))
	m.Update(release(x, y))
	if m.ShowHelp {
		t.Error("a click on help should close it")
	}
	// So does a click in the explorer; the top bar's Help button toggles.
	m.toggleHelp()
	m.Update(press(2, 3))
	m.Update(release(2, 3))
	if m.ShowHelp {
		t.Error("a click in the explorer should close help")
	}
	draw(m)
	time.Sleep(40 * time.Millisecond)
	draw(m)
	hz := m.Zones.Get("top_help")
	if hz == nil || hz.IsZero() {
		t.Fatal("no Help button")
	}
	m.Update(press(hz.StartX, 0))
	m.Update(release(hz.StartX, 0))
	if !m.ShowHelp {
		t.Error("clicking Help in the top bar should open help (and the release must not close it)")
	}
}

// In the code editor a drag is the editor's own selection; ctrl+c copies it
// and does not quit; with nothing selected ctrl+c quits.
func TestEditorDragThenCtrlC(t *testing.T) {
	m, hd := newTestScreen(t)
	md := filepath.Join(hd, "notes.md")
	os.WriteFile(md, []byte("hello world\n"), 0o644)
	m.openFileRaw(md)
	draw(m)
	time.Sleep(40 * time.Millisecond)
	draw(m)
	x, y := cellOf(t, m, "hello world")
	m.Update(press(x, y))
	m.Update(moveTo(x+5, y))
	m.Update(release(x+5, y))
	if m.frameSel.active {
		t.Fatal("a drag in the editor must not become a frame selection")
	}
	if got := m.TextEd.SelectedText(); got != "hello" {
		t.Fatalf("editor selected %q", got)
	}
	if _, cmd := ctrl(m, tea.KeyCtrlC); isQuit(cmd) {
		t.Fatal("ctrl+c with an editor selection quit the app")
	}
	if b, _ := os.ReadFile(md); string(b) != "hello world\n" {
		t.Errorf("copy changed the file: %q", b)
	}
	ctrl(m, tea.KeyRight) // collapse the selection
	if _, cmd := ctrl(m, tea.KeyCtrlC); !isQuit(cmd) {
		t.Error("ctrl+c without a selection quits")
	}
}

// Terminal panel: release keeps the selection and says ctrl+c copies; ctrl+c
// copies and never reaches the shell — even with no clipboard, where it used
// to fall through and interrupt whatever was running. With nothing selected
// ctrl+c does go to the shell.
func TestTerminalSelectionCtrlCNeverInterrupts(t *testing.T) {
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "shell-started")
	shell := filepath.Join(tmp, "fake-shell")
	// Any key forwarded to a stopped panel starts the shell; this one leaves
	// a mark so the test can see whether ctrl+c was forwarded.
	os.WriteFile(shell, []byte("#!/bin/sh\ntouch '"+marker+"'\nsleep 30\n"), 0o755)
	t.Setenv("SHELL", shell)

	m, hd := newTestScreen(t)
	t.Cleanup(m.Term.Close)
	m.openFileRaw(filepath.Join(hd, "a.hit"))
	m.Term.Open = true
	m.SetSize(m.Width, m.Height)
	draw(m)
	time.Sleep(40 * time.Millisecond)
	draw(m)
	tz := m.Zones.Get("term_view")
	if tz == nil || tz.IsZero() {
		t.Fatal("terminal panel has no zone")
	}
	m.Update(press(tz.StartX, tz.StartY))
	if !m.TermFocused {
		t.Fatal("a press in the panel focuses the terminal")
	}
	m.Update(moveTo(tz.StartX+8, tz.StartY))
	m.Update(release(tz.StartX+8, tz.StartY))
	if !m.Term.HasSelection() {
		t.Fatal("release dropped the terminal selection")
	}
	if m.StatusBar != "ctrl+c copies the selection" {
		t.Errorf("footer after release = %q", m.StatusBar)
	}
	if m.frameSel.active {
		t.Error("a drag in the terminal must not also select the frame")
	}

	_, cmd := ctrl(m, tea.KeyCtrlC)
	if isQuit(cmd) {
		t.Fatal("ctrl+c in the terminal never quits the app")
	}
	if !copied(m.StatusBar) {
		t.Errorf("status after ctrl+c = %q", m.StatusBar)
	}
	if m.Term.HasSelection() {
		t.Error("selection should clear once copied")
	}
	time.Sleep(300 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("ctrl+c with a selection was forwarded to the shell")
	}

	// No selection: ctrl+c belongs to the shell.
	ctrl(m, tea.KeyCtrlC)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("ctrl+c without a selection should reach the shell")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
