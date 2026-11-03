package screens

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hittable/shellapp/internal/extensions"
)

func TestApplyTextEdits(t *testing.T) {
	text := "a😀b\nsecond line\n"
	// Replace "b" (UTF-16 column 3, after the surrogate pair) and insert at
	// the start of line 2; applied from the end so both ranges stay valid.
	out, row, _ := applyTextEdits(text, []wireEdit{
		{Range: wireRange{wirePos{0, 3}, wirePos{0, 4}}, NewText: "B"},
		{Range: wireRange{wirePos{1, 0}, wirePos{1, 0}}, NewText: "> "},
	})
	if out != "a😀B\n> second line\n" || row != -1 {
		t.Errorf("got %q row %d", out, row)
	}
	// A snippet edit puts the cursor on its first tabstop.
	out, row, col := applyTextEdits("x\n", []wireEdit{{Range: wireRange{wirePos{0, 1}, wirePos{0, 1}}, NewText: "(${1:arg})", Snippet: true}})
	if out != "x(arg)\n" || row != 0 || col != 2 {
		t.Errorf("snippet: %q at %d:%d", out, row, col)
	}
	// Past-the-end positions clamp instead of panicking.
	if out, _, _ := applyTextEdits("ab", []wireEdit{{Range: wireRange{wirePos{0, 99}, wirePos{5, 0}}, NewText: "!"}}); out != "ab!" {
		t.Errorf("clamp: %q", out)
	}
}

func TestTeaKey(t *testing.T) {
	for in, want := range map[string]string{
		"alt+k": "alt+k", "ctrl+alt+k": "ctrl+alt+k", "alt+shift+x": "alt+X", "f6": "f6",
		"ctrl+shift+k": "", "cmd+k": "", "ctrl+k ctrl+c": "", "ctrl+k": "", "shift+f6": "",
	} {
		if got := teaKey(in); got != want {
			t.Errorf("teaKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWhenClauses(t *testing.T) {
	m, _ := newTestScreen(t)
	m.Focus = FocusTextEditor
	m.ExplorerFocused = false
	m.Ext.contexts["myExt.active"] = true
	for when, want := range map[string]bool{
		"":                                   true,
		"editorTextFocus":                    true,
		"editorLangId == yaml":               true,
		"editorLangId != yaml":               false,
		"editorTextFocus && myExt.active":    true,
		"!myExt.active":                      false,
		"unknownKey":                         false,
		"unknownKey || editorLangId == yaml": true,
		"editorLangId == 'yaml' && !unknown": true,
	} {
		if got := m.whenHolds(when, "yaml"); got != want {
			t.Errorf("whenHolds(%q) = %v, want %v", when, got, want)
		}
	}
}

// With an extension that needs Node installed but the folder untrusted,
// nothing starts, the footer says why, and picking an extension command from
// the palette offers to trust the folder.
func TestUntrustedFolderPausesExtensions(t *testing.T) {
	t.Setenv("HITTABLE_HOME", t.TempDir())
	vsix := filepath.Join(t.TempDir(), "c.vsix")
	f, _ := os.Create(vsix)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("extension/package.json")
	w.Write([]byte(`{"name":"c","publisher":"acme","version":"1.0.0","engines":{"vscode":"^1.80.0"},"main":"./m.js",
		"contributes":{"commands":[{"command":"acme.go","title":"Go Now","category":"Acme"}]}}`))
	w, _ = zw.Create("extension/m.js")
	w.Write([]byte(`exports.activate=()=>{}`))
	zw.Close()
	f.Close()
	if _, err := extensions.Install(context.Background(), vsix, nil); err != nil {
		t.Fatal(err)
	}

	m, _ := newTestScreen(t)
	m.SetSize(140, 30)
	if cmd := m.extStartCmd(); cmd != nil {
		t.Fatal("the host must not start in an untrusted folder")
	}
	if got := m.extFooter(); !strings.Contains(got, "folder not trusted") {
		t.Errorf("footer = %q", got)
	}

	m.openCommandPalette()
	if !m.Palette.ItemsOpen() {
		t.Fatal("command palette did not open")
	}
	if view := m.Palette.View(m.Zones); !strings.Contains(view, "Extensions: Install from Open VSX") {
		t.Errorf("palette is missing the built-in commands:\n%s", view)
	}
	for _, r := range "Go Now" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if view := m.Palette.View(m.Zones); !strings.Contains(view, "Acme: Go Now") {
		t.Errorf("palette is missing the contributed command:\n%s", view)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Palette.ItemsOpen() || !strings.Contains(m.Palette.ItemsTitle(), "Trust") {
		t.Fatalf("expected the trust prompt, got %q", m.Palette.ItemsTitle())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.Palette.Open {
		t.Error("esc should cancel the prompt")
	}
}

// A host request for a quick pick waits in the queue while another list is
// open, then shows.
func TestPromptsQueue(t *testing.T) {
	m, _ := newTestScreen(t)
	shown := 0
	first := func() {
		shown++
		m.showItems(paletteItems{Title: "first", Items: []paletteItem{{Label: "a"}}, OnPick: func(int, string, []int) { m.extNextPrompt() }})
	}
	second := func() {
		shown++
		m.showItems(paletteItems{Title: "second", Items: []paletteItem{{Label: "b"}}, OnPick: func(int, string, []int) { m.extNextPrompt() }})
	}
	m.extAsk(first)
	m.extAsk(second)
	if shown != 1 || m.Palette.ItemsTitle() != "first" {
		t.Fatalf("shown %d, title %q", shown, m.Palette.ItemsTitle())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if shown != 2 || m.Palette.ItemsTitle() != "second" {
		t.Fatalf("after the first: shown %d, title %q", shown, m.Palette.ItemsTitle())
	}
}
