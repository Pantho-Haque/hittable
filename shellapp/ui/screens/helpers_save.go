package screens

import (
	"encoding/json"

	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/hitfile"
)

// currentHit builds a HitFile from the Runner view widgets (raw, un-interpolated
// values) for the active .hit file, or nil if none is open. Invalid JSON in the
// Params/Headers tabs keeps the last good value instead of writing null.
func (m *MainScreen) currentHit() *hitfile.HitFile {
	if m.ActiveFile == "" {
		return nil
	}
	doc := m.Store.Get(m.ActiveFile)
	if doc == nil || doc.Kind != document.KindHit {
		return nil
	}
	existing, _ := doc.HitContent.(*hitfile.HitFile)
	if m.ViewMode == ViewText {
		var h hitfile.HitFile
		if err := json.Unmarshal([]byte(m.TextEd.GetContent()), &h); err != nil {
			return existing
		}
		return &h
	}
	method, url := m.URLBar.GetContent()
	h := &hitfile.HitFile{
		Method: method, URL: url,
		Headers: m.Headers.GetContent(), Params: m.Params.GetContent(),
		Body: m.Body.GetContent(),
	}
	if existing != nil {
		h.Response = existing.Response
		if h.Headers == nil {
			h.Headers = existing.Headers
		}
		if h.Params == nil {
			h.Params = existing.Params
		}
	}
	if h.Headers == nil {
		h.Headers = map[string]string{}
	}
	if h.Params == nil {
		h.Params = map[string]string{}
	}
	return h
}

// saveAndEnqueue commits the active view's content to its DocumentModel and
// schedules a debounced disk write.
func (m *MainScreen) saveAndEnqueue() {
	if m.ActiveFile == "" {
		return
	}
	doc := m.Store.Get(m.ActiveFile)
	if doc == nil || doc.Kind == document.KindBinary {
		return
	}

	var content string
	switch doc.Kind {
	case document.KindHit:
		if m.ViewMode == ViewText {
			content = m.TextEd.GetContent()
			if h := m.currentHit(); h != nil {
				doc.HitContent = h
			}
		} else {
			h := m.currentHit()
			if h == nil {
				return
			}
			content = string(hitfile.Marshal(h))
			doc.HitContent = h
		}
	case document.KindEnv:
		content = m.TextEd.GetContent()
		var env map[string]string
		if json.Unmarshal([]byte(content), &env) == nil && env != nil {
			m.EnvData = env
		}
	default:
		content = m.TextEd.GetContent()
	}

	doc.Lock()
	if content == doc.Content {
		doc.Unlock()
		return
	}
	doc.SetContent(content)
	gen := doc.IncGeneration()
	doc.SetLastFlushed(gen) // in sync with disk once the queue flushes
	doc.Unlock()
	m.WriteQueue.Update(doc.Path, content, gen)
	m.refreshGit(false)
}
