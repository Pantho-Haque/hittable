package screens

import (
	"encoding/json"
	"strings"

	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/hitfile"
)

func (m *MainScreen) saveAndEnqueue() {
	if m.ActiveFile == "" {
		return
	}
	doc := m.Store.Get(m.ActiveFile)
	if doc == nil {
		return
	}
	doc.Lock()
	defer doc.Unlock()

	var content string
	if doc.Kind == document.KindHit {
		method, _ := m.URLBar.GetContent()
		url := m.URLBar.URLInput.Value()
		hdrs := m.Headers.GetContent()
		params := m.Params.GetContent()
		bodyContent := m.Body.GetContent()
		h := &hitfile.HitFile{
			Method: method, URL: url, Headers: hdrs, Params: params, Body: bodyContent,
		}
		if doc.HitContent != nil {
			if existing, ok := doc.HitContent.(*hitfile.HitFile); ok && existing.Response != nil {
				h.Response = existing.Response
			}
		}
		data, _ := json.MarshalIndent(h, "", "  ")
		content = string(data) + "\n"
		doc.HitContent = h
	} else if doc.Kind == document.KindEnv {
		envText := m.TextEd.GetContent()
		env := make(map[string]string)
		for _, line := range strings.Split(envText, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				env[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		data, _ := json.MarshalIndent(env, "", "  ")
		content = string(data) + "\n"
		m.EnvData = env
	} else {
		content = m.TextEd.GetContent()
	}

	doc.SetContent(content)
	gen := doc.IncGeneration()
	doc.SetLastFlushed(gen)
	m.WriteQueue.Update(doc.Path, content, gen)
}
