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
