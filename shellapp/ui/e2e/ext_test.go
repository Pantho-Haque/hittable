package e2e

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hittable/shellapp/internal/extensions"
)

// buildVSIX zips a folder into a .vsix (the extension lives under extension/).
func buildVSIX(t *testing.T, src, dest string) {
	t.Helper()
	f, err := os.Create(dest)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	entries, _ := os.ReadDir(src)
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		w, _ := zw.Create("extension/" + e.Name())
		w.Write(b)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

// A real extension through the real terminal: installed from a .vsix, run
// by Node, its diagnostics in the editor and footer, its status item, its
// command from the command palette answering through a message box, its
// completions in the popup and its formatter on alt+shift+f.
func TestExtensionThroughRealTerminal(t *testing.T) {
	node := os.Getenv("HITTABLE_NODE")
	if node == "" {
		p, err := exec.LookPath("node")
		if err != nil {
			t.Skip("node is not installed")
		}
		node = p
	}
	t.Setenv("HITTABLE_NODE", node)
	t.Setenv("HITTABLE_HOME", t.TempDir())

	vsix := filepath.Join(t.TempDir(), "sample.vsix")
	buildVSIX(t, filepath.Join("..", "..", "internal", "exthost", "testdata", "sample"), vsix)
	if _, err := extensions.Install(context.Background(), vsix, nil); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := extensions.Trust(root); err != nil {
		t.Fatal(err)
	}
	notes := filepath.Join(root, "notes.txt")
	os.WriteFile(notes, []byte("first line   \nleft a TODO here\n"), 0o644)

	h, _ := startAt(t, root, 140, 40)
	dump := func() string { return strings.Join(h.screen(), "\n") }
	slow := func(s string) bool {
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if _, _, ok := h.find(s); ok {
				return true
			}
		}
		return false
	}

	x, y, ok := h.find("notes.txt")
	if !ok {
		t.Fatalf("explorer:\n%s", dump())
	}
	h.click(x, y)
	if !slow("sample ready") {
		t.Fatalf("status item from the extension not in the footer:\n%s", dump())
	}
	if !slow("⚠ 1") {
		t.Fatalf("problem count not in the footer:\n%s", dump())
	}

	// The cursor on the TODO line shows the diagnostic's message.
	fmt.Fprint(h.in, "\x1b[B") // down
	if !slow("a TODO is left") {
		t.Fatalf("diagnostic message not in the status row:\n%s", dump())
	}

	// Command palette → the extension's command → its message box.
	fmt.Fprint(h.in, "\x1bp") // alt+p
	if !slow("Command Palette") {
		t.Fatalf("command palette:\n%s", dump())
	}
	fmt.Fprint(h.in, "Say Hello")
	time.Sleep(200 * time.Millisecond)
	fmt.Fprint(h.in, "\r")
	if !slow("Hello from sample") {
		t.Fatalf("the extension's message box did not show:\n%s", dump())
	}
	fmt.Fprint(h.in, "\r") // OK
	if !h.gone("[Hello from sample]") {
		t.Fatalf("message box did not close:\n%s", dump())
	}

	// Completion from the extension while typing.
	fmt.Fprint(h.in, "\x1b[F") // end of line
	fmt.Fprint(h.in, " al")
	if !slow("alpha") {
		t.Fatalf("extension completion not offered:\n%s", dump())
	}
	fmt.Fprint(h.in, "\x1b") // close the popup
	time.Sleep(100 * time.Millisecond)

	// Format document: trailing spaces on line 1 go.
	fmt.Fprint(h.in, "\x1bF") // alt+shift+f
	fmt.Fprint(h.in, "\x13")  // ctrl+s
	deadline := time.Now().Add(10 * time.Second)
	for {
		b, _ := os.ReadFile(notes)
		if strings.HasPrefix(string(b), "first line\n") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("formatting not applied; file is %q\n%s", b, dump())
		}
		time.Sleep(100 * time.Millisecond)
	}
}
