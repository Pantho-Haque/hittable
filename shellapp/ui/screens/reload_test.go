package screens

import (
	"os"
	"path/filepath"
	"testing"

	zone "github.com/lrstanley/bubblezone"
)

func openScreen(t *testing.T) (*MainScreen, string) {
	t.Helper()
	tmp := t.TempDir()
	hd := filepath.Join(tmp, "hittable")
	os.MkdirAll(hd, 0o755)
	os.WriteFile(filepath.Join(hd, "env.json"), []byte(`{}`), 0o644)
	path := filepath.Join(hd, "notes.md")
	os.WriteFile(path, []byte("original\n"), 0o644)

	m := NewMainScreen(tmp, zone.New())
	m.SetSize(120, 40)
	m.openFileRaw(path)
	return m, path
}

// A git discard (or any write from outside the app) must reach the editor;
// the store never re-reads a path once it is open.
func TestExternalWriteIsAdopted(t *testing.T) {
	m, path := openScreen(t)
	if got := m.TextEd.GetContent(); got != "original\n" {
		t.Fatalf("editor opened with %q", got)
	}

	os.WriteFile(path, []byte("from git\n"), 0o644)
	m.reloadExternalEdits()

	if got := m.TextEd.GetContent(); got != "from git\n" {
		t.Errorf("editor still shows %q", got)
	}
	if doc := m.Store.Get(path); doc == nil || doc.GetContent() != "from git\n" {
		t.Error("document model still holds the stale content")
	}

	// Typing afterwards must build on the new content, not resurrect the old
	// buffer over the top of the discard.
	m.TextEd.SetContent(path, "from git\nmore\n")
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow()
	if b, _ := os.ReadFile(path); string(b) != "from git\nmore\n" {
		t.Errorf("autosave wrote a stale buffer: %q", b)
	}
}

// Unsaved local edits win: adopting disk content there would lose typing.
func TestUnflushedEditsAreNotClobbered(t *testing.T) {
	m, path := openScreen(t)

	m.TextEd.SetContent(path, "my typing\n")
	m.saveAndEnqueue() // marks the doc dirty until the queue flushes
	doc := m.Store.Get(path)
	doc.Lock()
	doc.LastFlushed = doc.Generation - 1 // pretend the flush has not happened
	doc.Unlock()

	os.WriteFile(path, []byte("from elsewhere\n"), 0o644)
	m.reloadExternalEdits()

	if got := m.TextEd.GetContent(); got != "my typing\n" {
		t.Errorf("unsaved edit was replaced by disk content: %q", got)
	}
}

// Our own write must not look like an external change.
func TestOwnWriteIsNotReloaded(t *testing.T) {
	m, path := openScreen(t)
	m.TextEd.SetContent(path, "mine\n")
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow()

	m.reloadExternalEdits()
	if got := m.TextEd.GetContent(); got != "mine\n" {
		t.Errorf("own write was treated as external: %q", got)
	}
	if m.StatusBar != "" {
		t.Errorf("reported a reload for our own write: %q", m.StatusBar)
	}
}
