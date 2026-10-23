package screens

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/hitfile"
	"github.com/hittable/shellapp/ui/components/texteditor"
)

// The same checks as the web app's tests/e2e/files.spec.ts: big source files,
// images, binaries, and .hit files kept in step between Runner and Text.

const png3x2 = "iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAIAAAASFvFNAAAAEElEQVR4nGP4z8AAQQxwFgBB0gX7h/C5SAAAAABJRU5ErkJggg=="

// waitDisk does what the autosave debounce does in the running app (commit
// the view, flush the queue), then checks the file. That autosave fires by
// itself is covered on the real binary in cmd/hittable/realterm_test.go.
func waitDisk(t *testing.T, m *MainScreen, path string, ok func(string) bool) string {
	t.Helper()
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow()
	deadline := time.Now().Add(3 * time.Second)
	for {
		b, _ := os.ReadFile(path)
		if ok(string(b)) {
			return string(b)
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never reached the expected content; it holds:\n%.300s", filepath.Base(path), b)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestBigSourceFileOpensEditsAndSavesQuickly(t *testing.T) {
	m, hd := newTestScreen(t)
	var sb strings.Builder
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&sb, "func f%d() int { return %d } // line %d\n", i, i, i+1)
	}
	p := filepath.Join(hd, "big.go")
	os.WriteFile(p, []byte(sb.String()), 0o644)

	start := time.Now()
	m.openFileRaw(p)
	_ = m.View()
	if d := time.Since(start); d > time.Second*raceSlowdown {
		t.Fatalf("opening 5,000 lines took %v", d)
	}
	start = time.Now()
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	runes(m, "// added")
	_ = m.View()
	if d := time.Since(start); d > time.Second*raceSlowdown {
		t.Fatalf("jumping to the end and typing took %v", d)
	}
	got := waitDisk(t, m, p, func(s string) bool { return strings.Contains(s, "// added") })
	if !strings.HasPrefix(got, "func f0() int { return 0 } // line 1\n") || strings.Index(got, "// added") < strings.Index(got, "// line 5000") {
		t.Fatalf("edit landed in the wrong place or damaged the file")
	}
}

func TestFileKindsMatchTheWebApp(t *testing.T) {
	m, hd := newTestScreen(t)
	img, _ := base64.StdEncoding.DecodeString(png3x2)
	bin := []byte{0x7f, 0x45, 0x4c, 0x46, 0, 0, 0, 1, 2, 3, 0, 255}
	huge := strings.Repeat("x", texteditor.MaxEditableBytes+10)
	files := map[string][]byte{"logo.png": img, "tool": bin, "huge.log": []byte(huge), "badge.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`)}
	for name, data := range files {
		os.WriteFile(filepath.Join(hd, name), data, 0o644)
	}
	for _, c := range []struct{ name, label string }{
		{"logo.png", "PNG image · 3 × 2 · 73 B"},
		{"tool", "Binary file · 12 B"},
		{"huge.log", "Too large to edit · 1.0 MB"},
	} {
		p := filepath.Join(hd, c.name)
		m.openFileRaw(p)
		if doc := m.Store.Get(p); doc == nil || doc.Kind != document.KindBinary {
			t.Fatalf("%s should not open in the editor", c.name)
		}
		if !strings.Contains(m.StatusBar, c.label) || !strings.Contains(m.View(), c.label) {
			t.Errorf("%s: want %q on screen, status %q", c.name, c.label, m.StatusBar)
		}
		// Keys a user might press while it is open never write it.
		runes(m, "oops")
		ctrl(m, tea.KeyCtrlS)
		m.WriteQueue.FlushNow()
		if b, _ := os.ReadFile(p); string(b) != string(files[c.name]) {
			t.Fatalf("%s was rewritten", c.name)
		}
	}
	// An SVG is text: it opens in the editor (the web app adds a picture preview).
	svg := filepath.Join(hd, "badge.svg")
	m.openFileRaw(svg)
	if !strings.Contains(m.TextEd.GetContent(), "<svg") {
		t.Fatalf("SVG should open as code, editor holds %q", m.TextEd.GetContent())
	}
}

func TestHitRunnerAndTextStayInStep(t *testing.T) {
	m, hd := newTestScreen(t)
	hit := filepath.Join(hd, "req.hit")

	t.Run("a big .hit still opens, in the Runner", func(t *testing.T) {
		writeHit(t, hit, hitfile.HitFile{Method: "POST", URL: "http://x", Body: strings.Repeat("b", texteditor.MaxEditableBytes+5)})
		before, _ := os.ReadFile(hit)
		m.openFileRaw(hit)
		if m.ViewMode != ViewRunner || len(m.Body.GetContent()) != texteditor.MaxEditableBytes+5 {
			t.Fatalf("view=%d body=%d", m.ViewMode, len(m.Body.GetContent()))
		}
		m.closeFile()
		m.WriteQueue.FlushNow()
		if after, _ := os.ReadFile(hit); string(after) != string(before) {
			t.Fatalf("opening and closing rewrote it:\nbefore %.120q\nafter  %.120q", before, after)
		}
	})

	t.Run("opening and switching views never rewrites the file", func(t *testing.T) {
		custom := `{"method":"GET","url":"http://x/users/1","headers":{},"params":{},"body":"","response":null}`
		os.WriteFile(hit, []byte(custom), 0o644)
		m.Store.Delete(hit)
		m.openFileRaw(hit)
		if m.ViewMode != ViewRunner {
			t.Fatalf("a valid .hit should open in the Runner, view=%d", m.ViewMode)
		}
		ctrl(m, tea.KeyCtrlT)
		ctrl(m, tea.KeyCtrlT)
		m.closeFile()
		m.WriteQueue.FlushNow()
		if b, _ := os.ReadFile(hit); string(b) != custom {
			t.Fatalf("file rewritten without an edit (noise in git):\n%.200s", b)
		}
	})

	t.Run("a Runner edit reaches the disk and shows in Text", func(t *testing.T) {
		m.openFileRaw(hit)
		m.Focus = FocusURLBar
		m.focusCurrent()
		m.Update(tea.KeyMsg{Type: tea.KeyEnd})
		runes(m, "9")
		waitDisk(t, m, hit, func(s string) bool { return strings.Contains(s, "http://x/users/19") })
		ctrl(m, tea.KeyCtrlT)
		if !strings.Contains(m.TextEd.GetContent(), "http://x/users/19") {
			t.Fatalf("Text view is behind the Runner: %s", m.TextEd.GetContent())
		}
	})

	t.Run("a Text edit reaches the disk and shows in the Runner", func(t *testing.T) {
		if m.ViewMode != ViewText {
			ctrl(m, tea.KeyCtrlT)
		}
		m.Focus = FocusTextEditor
		m.focusCurrent()
		ctrl(m, tea.KeyCtrlA)
		typeText(m, `{"method":"DELETE","url":"http://x/users/9","headers":{"X-Keep":"1"},"params":{},"body":"","response":null}`)
		waitDisk(t, m, hit, func(s string) bool { return strings.Contains(s, `"DELETE"`) })
		ctrl(m, tea.KeyCtrlT)
		method, url := m.URLBar.GetContent()
		if m.ViewMode != ViewRunner || method != "DELETE" || !strings.Contains(url, "/users/9") {
			t.Fatalf("Runner is behind Text: %s %s", method, url)
		}
	})

	t.Run("half-typed headers never wipe the saved ones, even across a view switch", func(t *testing.T) {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2"), Alt: true}) // Headers tab
		ctrl(m, tea.KeyCtrlA)
		typeText(m, `{"X-Broken": `)
		m.saveAndEnqueue() // what the autosave debounce does
		m.WriteQueue.FlushNow()
		if h, _ := readHit(t, hit)["headers"].(map[string]any); h["X-Keep"] != "1" {
			t.Fatalf("headers wiped on disk: %v", readHit(t, hit)["headers"])
		}
		ctrl(m, tea.KeyCtrlT)
		if !strings.Contains(m.TextEd.GetContent(), `"X-Keep": "1"`) {
			t.Fatalf("Text view lost the last valid headers: %s", m.TextEd.GetContent())
		}
	})
}
