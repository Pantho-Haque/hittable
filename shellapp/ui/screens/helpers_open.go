package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/hitfile"
	"github.com/hittable/shellapp/ui/components/texteditor"
)

// detectBinary returns true if buf contains a null byte in its first 8KB.
func detectBinary(buf []byte) bool {
	if len(buf) > 8192 {
		buf = buf[:8192]
	}
	for _, b := range buf {
		if b == 0 {
			return true
		}
	}
	return false
}

// humanSize formats a byte count as B / KB / MB.
func humanSize(n int) string {
	const k = 1024
	if n < k {
		return fmt.Sprintf("%d B", n)
	}
	if n < k*k {
		return fmt.Sprintf("%.1f KB", float64(n)/k)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(k*k))
}

// loadDoc returns the session's DocumentModel for path, reading disk only the
// first time the path is opened.
func (m *MainScreen) loadDoc(path string) (*document.DocumentModel, error) {
	if doc := m.Store.Get(path); doc != nil {
		return doc, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	kind := document.KindGeneric
	switch {
	case filepath.Ext(path) == ".hit":
		kind = document.KindHit
	case path == m.EnvPath:
		kind = document.KindEnv
	case filepath.Ext(path) == ".md":
		kind = document.KindMarkdown
	}
	if kind == document.KindGeneric || kind == document.KindMarkdown {
		if len(content) > texteditor.MaxEditableBytes {
			kind = document.KindBinary
			m.StatusBar = fmt.Sprintf("File too large (%s) — preview disabled", humanSize(len(content)))
		} else if detectBinary(content) {
			kind = document.KindBinary
			m.StatusBar = fmt.Sprintf("Binary file (%s) — preview disabled", humanSize(len(content)))
		}
	}
	doc := m.Store.GetOrCreate(path, kind, string(content), content)
	if fi, err := os.Stat(path); err == nil {
		doc.Lock()
		doc.DiskMod = fi.ModTime()
		doc.Unlock()
	}
	if kind == document.KindHit {
		var h hitfile.HitFile
		if err := json.Unmarshal(content, &h); err == nil {
			doc.HitContent = &h
		}
	}
	return doc, nil
}

func (m *MainScreen) openFileRaw(path string) {
	if m.ActiveFile == path {
		if m.ViewMode == ViewText {
			m.Focus = FocusTextEditor
		} else if m.Focus == FocusExplorerPane {
			m.Focus = FocusURLBar
		}
		m.focusCurrent()
		return
	}
	m.saveAndEnqueue() // commit the outgoing file before switching
	m.StatusBar = ""

	doc, err := m.loadDoc(path)
	if err != nil {
		m.StatusBar = "Error opening " + filepath.Base(path)
		return
	}
	m.ActiveFile = path
	m.ShowHelp = false
	m.GitOpen = false
	m.Git.SetActiveFile(path)
	m.applyBlame()
	m.URLBar.CloseDropdown()
	m.Response.CloseSearch()
	m.ActiveTab = 0

	if doc.Kind == document.KindHit {
		if h, ok := doc.HitContent.(*hitfile.HitFile); ok {
			m.loadHitIntoRunner(h)
			m.ViewMode = ViewRunner
			doc.ViewMode = document.ViewRunner
			m.Focus = FocusURLBar
			m.focusCurrent()
			return
		}
		m.StatusBar = "Invalid .hit JSON — opened as text"
	}

	m.ViewMode = ViewText
	doc.ViewMode = document.ViewText
	if doc.Kind == document.KindBinary {
		m.TextEd.SetContent(path, "")
		m.Focus = FocusExplorerPane
		m.setExplorerFocused(true)
		return
	}
	m.TextEd.Wrap = doc.Kind == document.KindMarkdown
	m.TextEd.SetContent(path, doc.Content)
	m.Focus = FocusTextEditor
	m.focusCurrent()
	if doc.Kind == document.KindMarkdown {
		m.setMdMode(m.MdMode) // re-layout for the remembered Text/Preview/Split choice
	}
}

// reloadExternalEdits re-reads any open document whose file changed on disk
// underneath us: a git discard or checkout, a command in the integrated
// terminal, another editor. The store deliberately never re-reads a path once
// it is open, so without this the editor keeps showing stale content — and the
// next autosave writes that content back, silently undoing the change on disk.
//
// A document with edits that have not reached the write queue yet is left
// alone; adopting disk content there would throw away what the user just
// typed.
//
// ponytail: stat per tick over the open documents, so an edit made outside the
// app shows up within the poll interval. Reach for fsnotify if that ever needs
// to be instant.
func (m *MainScreen) reloadExternalEdits() {
	for _, path := range m.Store.Paths() {
		doc := m.Store.Get(path)
		if doc == nil || doc.Kind == document.KindBinary {
			continue
		}
		fi, err := os.Stat(path)
		if err != nil {
			continue // deleted or unreadable; the explorer reports that
		}
		doc.Lock()
		// Size is checked too: a coarse filesystem clock can leave the mtime
		// unchanged for a write that lands in the same tick.
		unchanged := fi.ModTime().Equal(doc.DiskMod) && fi.Size() == int64(len(doc.Content))
		dirty := doc.GetGeneration() != doc.GetLastFlushed()
		doc.DiskMod = fi.ModTime()
		doc.Unlock()
		if unchanged || dirty {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		doc.Lock()
		same := string(content) == doc.Content
		doc.Unlock()
		if same {
			continue // our own write landing, or a touch that changed nothing
		}
		m.adoptDiskContent(doc, content)
	}
}

// adoptDiskContent replaces a document's content with what is on disk and
// refreshes whatever is showing it.
func (m *MainScreen) adoptDiskContent(doc *document.DocumentModel, content []byte) {
	m.WriteQueue.Cancel(doc.Path) // a queued stale write must not land after this
	doc.Lock()
	doc.SetContent(string(content))
	doc.RawContent = content
	doc.SetLastFlushed(doc.GetGeneration())
	doc.Unlock()
	if doc.Kind == document.KindHit {
		var h hitfile.HitFile
		if err := json.Unmarshal(content, &h); err == nil {
			doc.HitContent = &h
		}
	}
	if doc.Path != m.ActiveFile {
		return // reopening it will show the new content
	}
	if doc.Kind == document.KindHit && m.ViewMode == ViewRunner {
		if h, ok := doc.HitContent.(*hitfile.HitFile); ok {
			m.loadHitIntoRunner(h)
		}
		return
	}
	m.TextEd.SetContent(doc.Path, doc.GetContent()) // keeps the cursor where it was
	if doc.Kind == document.KindMarkdown {
		m.Preview.SetContent(doc.GetContent())
	}
	m.StatusBar = "Reloaded " + filepath.Base(doc.Path) + " — changed on disk"
}
