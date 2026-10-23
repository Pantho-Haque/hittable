package palette

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFindFileAndGrep(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src", "node_modules", "x"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "main_handler.go"), []byte("package x\nfunc Serve() {}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "src", "util.go"), []byte("package x\n// serve helper\n"), 0o644)
	os.WriteFile(filepath.Join(root, "src", "node_modules", "x", "index.js"), []byte("serve()"), 0o644)

	var got chan ResultsMsg = make(chan ResultsMsg, 4)
	p := New(root, func(m tea.Msg) { got <- m.(ResultsMsg) })
	p.SetSize(80, 20)
	p.Show(ModeFiles)
	if len(p.Results) != 2 {
		t.Fatalf("index should skip node_modules: %+v", p.Results)
	}
	for _, r := range "mh" {
		p.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(p.Results) == 0 || p.Results[0].Rel != "src/main_handler.go" {
		t.Errorf("fuzzy: %+v", p.Results)
	}

	p.Show(ModeGrep)
	for _, r := range "serve" {
		p.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	var last ResultsMsg
	deadline := time.After(5 * time.Second)
	for last.Seq != p.seq {
		select {
		case last = <-got:
		case <-deadline:
			t.Fatal("no grep results")
		}
	}
	p.Deliver(last)
	if len(p.Results) != 2 {
		t.Fatalf("grep results: %+v", p.Results)
	}
	for _, r := range p.Results {
		if r.Line == 0 || r.Rel == "" {
			t.Errorf("bad result: %+v", r)
		}
	}
	var opened Result
	p.OnOpen = func(r Result) { opened = r }
	p.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if opened.Path == "" || p.Open {
		t.Errorf("enter should open and close: %+v open=%v", opened, p.Open)
	}
	if v := p.View(zoneManager()); v == "" {
		t.Error("empty view")
	}
}

// With no results, paging or scrolling down then pressing Enter used to read
// Results[-1] and crash the app (found by a keyboard-mash test).
func TestEmptyResultsNeverCrash(t *testing.T) {
	p := New(t.TempDir(), func(tea.Msg) {})
	opened := false
	p.OnOpen = func(Result) { opened = true }
	for _, k := range []tea.KeyType{tea.KeyPgDown, tea.KeyDown, tea.KeyPgUp, tea.KeyUp, tea.KeyEnter} {
		p.HandleKey(tea.KeyMsg{Type: k})
	}
	p.HandleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress, Type: tea.MouseWheelDown})
	p.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if p.Cursor != 0 || opened {
		t.Fatalf("cursor = %d, opened = %v; want 0, false", p.Cursor, opened)
	}
}
