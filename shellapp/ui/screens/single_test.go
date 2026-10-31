package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
)

// `hittable <file>`: the file and the terminal, and nothing that browses the
// project — no explorer, Git or Find, by key or by button.
func TestSingleFileModeShowsOnlyTheFile(t *testing.T) {
	tmp := t.TempDir()
	hd := filepath.Join(tmp, "hittable")
	os.MkdirAll(filepath.Join(hd, "api"), 0o755)
	os.WriteFile(filepath.Join(hd, "env.json"), []byte(`{"BASE_URL":"http://x.test"}`), 0o644)
	hit := filepath.Join(hd, "api", "a.hit")
	os.WriteFile(hit, []byte(`{"method":"GET","url":"<<BASE_URL>>/a","headers":{},"params":{},"body":"","response":null}`), 0o644)
	txt := filepath.Join(tmp, "notes.txt")
	os.WriteFile(txt, []byte("alpha beta\n"), 0o644)

	m := NewMainScreen(filepath.Dir(hit), zone.New())
	m.SetSize(120, 30)
	m.EnterSingleFile(hit)

	if m.ActiveFile != hit || m.ViewMode != ViewRunner {
		t.Fatalf("a .hit file should open in the runner: active=%q mode=%v", m.ActiveFile, m.ViewMode)
	}
	if m.EnvData["BASE_URL"] != "http://x.test" {
		t.Errorf("env.json above the file was not found: %v", m.EnvData)
	}
	if !m.explorerHidden() || m.MainWidth != m.Width {
		t.Error("the explorer is still taking space")
	}
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyCtrlB}, {Type: tea.KeyF5}, {Type: tea.KeyCtrlP},
		{Type: tea.KeyRunes, Runes: []rune("g"), Alt: true}, {Type: tea.KeyEsc},
	} {
		m.Update(k)
		if m.GitOpen || m.Palette.Open || m.ExplorerFocused || m.ActiveFile != hit {
			t.Fatalf("%q escaped single-file mode: git=%v find=%v explorer=%v file=%q",
				k.String(), m.GitOpen, m.Palette.Open, m.ExplorerFocused, m.ActiveFile)
		}
	}
	top := ansi.Strip(m.renderTopBar())
	if strings.Contains(top, "Git") || strings.Contains(top, "Find") || !strings.Contains(top, "a.hit") {
		t.Errorf("top bar: %q", top)
	}

	// Any other file is just the editor.
	m2 := NewMainScreen(tmp, zone.New())
	m2.SetSize(120, 30)
	m2.EnterSingleFile(txt)
	if m2.ViewMode != ViewText || m2.Focus != FocusTextEditor {
		t.Fatalf("plain file: mode=%v focus=%v", m2.ViewMode, m2.Focus)
	}
	m2.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if !m2.TextEd.PromptOpen() {
		t.Error("ctrl+f did not open the find bar")
	}
}
