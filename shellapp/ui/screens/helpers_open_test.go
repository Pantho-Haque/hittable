package screens

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/hittable/shellapp/internal/document"
)

// TestDetectBinary asserts the null-byte heuristic: any null byte in the
// first 8KB is treated as binary. This is the same rule `git` uses.
func TestDetectBinary(t *testing.T) {
	if detectBinary([]byte("plain text\n")) {
		t.Errorf("plain text should not be binary")
	}
	if !detectBinary([]byte{0x89, 'P', 'N', 'G', 0, 0x0D, 0x0A}) {
		t.Errorf("PNG header should be detected as binary")
	}
	if !detectBinary([]byte{'a', 'b', 'c', 0x00, 'd'}) {
		t.Errorf("embedded null byte should be detected as binary")
	}
	// Null byte past the 8KB window — should NOT be flagged (we only
	// inspect the first 8KB for performance).
	buf := make([]byte, 8193)
	for i := range buf {
		buf[i] = 'a'
	}
	buf[8192] = 0
	if detectBinary(buf) {
		t.Errorf("null byte past first 8KB should not be detected as binary")
	}
}

func TestHumanSize(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{2 * 1024 * 1024, "2.0 MB"},
	}
	for _, c := range cases {
		if got := humanSize(c.n); got != c.want {
			t.Errorf("humanSize(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

// TestBinaryFileOpensAsPlaceholder: a binary file should be classified as
// KindBinary, the text editor should NOT receive the file contents, and the
// StatusBar should report the size.
func TestBinaryFileOpensAsPlaceholder(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	// Create a "binary" file (PNG header) at <tmpDir>/foo.bin
	binaryPath := filepath.Join(tmpDir, "foo.bin")
	os.WriteFile(binaryPath, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}, 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(120, 30)

	m.OpenFile(binaryPath)

	doc := m.Store.Get(binaryPath)
	if doc == nil {
		t.Fatalf("expected doc to be created")
	}
	if doc.Kind != document.KindBinary {
		t.Errorf("expected KindBinary, got %d", doc.Kind)
	}
	if got := m.TextEd.GetContent(); got != "" {
		t.Errorf("expected empty text editor, got %q", got)
	}
	if m.StatusBar == "" {
		t.Error("expected StatusBar to mention binary file")
	}
	// View() should render the placeholder, not crash.
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

// TestLargeFileOpensAsPlaceholder: a text file larger than the editor cap
// should also be classified as KindBinary.
func TestLargeFileOpensAsPlaceholder(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	largePath := filepath.Join(tmpDir, "big.txt")
	// 2 MB of plain ASCII text — over the 1 MB cap.
	chunk := []byte("hello world\n")
	big := make([]byte, 0, 2*1024*1024)
	for len(big) < 2*1024*1024 {
		big = append(big, chunk...)
	}
	os.WriteFile(largePath, big, 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(120, 30)

	m.OpenFile(largePath)

	doc := m.Store.Get(largePath)
	if doc == nil {
		t.Fatalf("expected doc to be created")
	}
	if doc.Kind != document.KindBinary {
		t.Errorf("expected KindBinary for oversized file, got %d", doc.Kind)
	}
	_ = tea.KeyMsg{}
}
