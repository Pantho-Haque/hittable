package exthost

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hittable/shellapp/internal/extensions"
)

// TestRealExtensions installs real extensions from Open VSX and drives them:
// Red Hat YAML (a language server behind vscode-languageclient) and the
// Dracula theme. It needs the network, so it only runs with
// HITTABLE_EXT_E2E=1.
func TestRealExtensions(t *testing.T) {
	if os.Getenv("HITTABLE_EXT_E2E") == "" {
		t.Skip("set HITTABLE_EXT_E2E=1 to install real extensions from Open VSX")
	}
	node := nodeOrSkip(t)
	t.Setenv("HITTABLE_HOME", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	res, err := extensions.Install(ctx, "redhat.vscode-yaml", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Installed.Signed {
		t.Error("redhat.vscode-yaml should be signature-verified")
	}
	if _, err := extensions.Install(ctx, "dracula-theme.theme-dracula", nil); err != nil {
		t.Fatal(err)
	}
	themes := extensions.Themes()
	if len(themes) == 0 {
		t.Fatal("no themes from the Dracula extension")
	}
	if st, _, err := themes[0].ChromaStyle(); err != nil || st == nil {
		t.Fatalf("theme conversion: %v", err)
	}

	root := t.TempDir()
	file := filepath.Join(root, "config.yaml")
	text := "name: demo\nitems:\n  - a\n  - b\nbad: [unclosed\n"
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := &recorder{notes: map[string][]json.RawMessage{}}
	h, err := Start(ctx, Options{
		Node: node, Root: root, Extensions: extensions.Enabled(),
		OnRequest: func(r *Request) {
			go func() {
				if r.Method == "languages/idFor" {
					var p struct{ Path string }
					_ = json.Unmarshal(r.Params, &p)
					r.Reply(extensions.LanguageID(p.Path), nil)
					return
				}
				r.Reply(nil, nil)
			}()
		},
		OnNotify: rec.notify,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	if err := h.Call(ctx, "doc/open", map[string]any{"path": file, "languageId": "yaml", "version": 1, "text": text}, nil); err != nil {
		t.Fatal(err)
	}
	rec.waitFor(t, "ext/activated", func(p json.RawMessage) bool { return strings.Contains(string(p), "redhat.vscode-yaml") })

	// The server reports the unclosed flow sequence.
	deadline := time.Now().Add(60 * time.Second)
	found := false
	for time.Now().Before(deadline) && !found {
		rec.mu.Lock()
		for _, p := range rec.notes["diagnostics/set"] {
			if strings.Contains(string(p), "config.yaml") && strings.Contains(string(p), `"message"`) {
				found = true
			}
		}
		rec.mu.Unlock()
		time.Sleep(200 * time.Millisecond)
	}
	if !found {
		rec.mu.Lock()
		t.Fatalf("no YAML diagnostics; logs: %s", rec.notes["log"])
	}

	var hover struct{ Markdown string }
	_ = h.Call(ctx, "provide/hover", map[string]any{"path": file, "position": map[string]int{"line": 0, "character": 1}}, &hover)

	var provs map[string][]string
	for i := 0; i < 50; i++ {
		_ = h.Call(ctx, "debug/providers", nil, &provs)
		if provs["formatting"] != nil {
			t.Logf("formatter registered after %d polls", i)
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Logf("providers: %d kinds", len(provs))
	rec.mu.Lock()
	for _, l := range rec.notes["log"] {
		t.Logf("log: %.300s", l)
	}
	for _, l := range rec.notes["output/append"] {
		t.Logf("out: %.300s", l)
	}
	rec.mu.Unlock()
	var edits []json.RawMessage
	fixed := "name:    demo\nitems:\n  - a\n  - b\n"
	_ = h.Call(ctx, "doc/change", map[string]any{"path": file, "version": 2, "text": fixed}, nil)
	if err := h.Call(ctx, "provide/formatting", map[string]any{"path": file}, &edits); err != nil {
		t.Fatal(err)
	}
	if len(edits) == 0 {
		t.Errorf("the YAML formatter returned no edits for %q", fixed)
	}
	t.Logf("hover=%q formatting edits=%d", hover.Markdown, len(edits))
	rec.mu.Lock()
	for _, l := range rec.notes["log"] {
		if strings.Contains(string(l), "does not support") {
			t.Logf("unsupported API: %s", l)
		}
	}
	rec.mu.Unlock()
}

// TestRealPrettier formats JavaScript with the Prettier extension, which
// formats in-process (no language server) and reads its options through
// workspace.getConfiguration.
func TestRealPrettier(t *testing.T) {
	if os.Getenv("HITTABLE_EXT_E2E") == "" {
		t.Skip("set HITTABLE_EXT_E2E=1 to install real extensions from Open VSX")
	}
	node := nodeOrSkip(t)
	t.Setenv("HITTABLE_HOME", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if _, err := extensions.Install(ctx, "esbenp.prettier-vscode", nil); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	file := filepath.Join(root, "a.js")
	text := "const a  =  {b:1,c:[1,2 ,3]}\n"
	_ = os.WriteFile(file, []byte(text), 0o644)
	rec := &recorder{notes: map[string][]json.RawMessage{}}
	h, err := Start(ctx, Options{Node: node, Root: root, Extensions: extensions.Enabled(),
		OnRequest: func(r *Request) {
			go func() {
				var p struct{ Path string }
				_ = json.Unmarshal(r.Params, &p)
				if r.Method == "languages/idFor" {
					r.Reply(extensions.LanguageID(p.Path), nil)
					return
				}
				r.Reply(nil, nil)
			}()
		},
		OnNotify: rec.notify})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	_ = h.Call(ctx, "doc/open", map[string]any{"path": file, "languageId": "javascript", "version": 1, "text": text}, nil)
	var edits []struct {
		Range   struct{ Start, End struct{ Line, Character int } }
		NewText string
	}
	for i := 0; i < 100; i++ {
		_ = h.Call(ctx, "provide/formatting", map[string]any{"path": file}, &edits)
		if len(edits) > 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if len(edits) == 0 {
		rec.mu.Lock()
		t.Fatalf("Prettier returned no edits; logs: %s\noutput: %s", rec.notes["log"], rec.notes["output/append"])
	}
	// One-line ASCII input: apply the edits from the end by column.
	got := strings.TrimSuffix(text, "\n")
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		if e.Range.Start.Line != 0 || e.Range.End.Line != 0 {
			got = "multi-line edits: " + got
			break
		}
		got = got[:e.Range.Start.Character] + e.NewText + got[e.Range.End.Character:]
	}
	if !strings.Contains(got, "const a = { b: 1, c: [1, 2, 3] };") {
		t.Errorf("Prettier output: %q", got)
	}
}
