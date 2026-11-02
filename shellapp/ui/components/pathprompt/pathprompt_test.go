package pathprompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func typeIn(p *Prompt, s string) {
	p.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

func key(p *Prompt, k tea.KeyType) (string, bool) { return p.HandleKey(tea.KeyMsg{Type: k}) }

func TestTabCompletesFoldersAndFiles(t *testing.T) {
	base := t.TempDir()
	os.MkdirAll(filepath.Join(base, "files", "img"), 0o755)
	os.WriteFile(filepath.Join(base, "files", "report.pdf"), nil, 0o644)
	os.WriteFile(filepath.Join(base, "files", "readme.md"), nil, 0o644)
	os.WriteFile(filepath.Join(base, "files", ".hidden"), nil, 0o644)

	var p Prompt
	p.Show(base)
	typeIn(&p, "fi")
	key(&p, tea.KeyTab)
	if p.Value != "files/" {
		t.Fatalf("a folder completes with a slash: %q", p.Value)
	}
	typeIn(&p, "re")
	key(&p, tea.KeyTab)
	if p.Value != "files/re" || !strings.Contains(p.View(200), "readme.md  report.pdf") {
		t.Fatalf("two matches stop at the shared prefix and are listed: %q %q", p.Value, p.View(200))
	}
	typeIn(&p, "p")
	key(&p, tea.KeyTab)
	if p.Value != "files/report.pdf" {
		t.Fatalf("one match completes: %q", p.Value)
	}
	if got, ok := key(&p, tea.KeyEnter); !ok || got != filepath.Join(base, "files", "report.pdf") || p.Open {
		t.Fatalf("enter returns the absolute path: %q %v", got, ok)
	}

	// Hidden files only when asked for; ctrl+w goes up a folder.
	if _, m := Complete(base, "files/"); len(m) != 3 {
		t.Errorf("hidden files listed without a dot: %v", m)
	}
	if _, m := Complete(base, "files/."); len(m) != 1 {
		t.Errorf("a dot prefix shows hidden files: %v", m)
	}
	p.Show(base)
	typeIn(&p, "files/img/")
	key(&p, tea.KeyCtrlW)
	if p.Value != "files/" {
		t.Errorf("ctrl+w: %q", p.Value)
	}
}

func TestHomeExpansion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, "Pictures"), 0o755)
	os.WriteFile(filepath.Join(home, "Pictures", "me.png"), nil, 0o644)

	if v, _ := Complete("/elsewhere", "~"); v != "~/" {
		t.Errorf("~ completes to ~/: %q", v)
	}
	if v, _ := Complete("/elsewhere", "~/Pic"); v != "~/Pictures/" {
		t.Errorf("~/ completes from home: %q", v)
	}
	var p Prompt
	p.Show("/elsewhere")
	typeIn(&p, "~/Pictures/me.png")
	if got, ok := key(&p, tea.KeyEnter); !ok || got != filepath.Join(home, "Pictures", "me.png") {
		t.Errorf("~ expands on enter: %q", got)
	}
}

func TestEscCancels(t *testing.T) {
	var p Prompt
	p.Show(t.TempDir())
	typeIn(&p, "abc")
	if got, ok := key(&p, tea.KeyEsc); ok || got != "" || p.Open {
		t.Errorf("esc: %q %v open=%v", got, ok, p.Open)
	}
	p.Show(t.TempDir())
	if _, ok := key(&p, tea.KeyEnter); ok || !p.Open {
		t.Error("enter with nothing typed does nothing")
	}
}
