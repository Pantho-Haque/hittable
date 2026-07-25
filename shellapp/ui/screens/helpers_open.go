package screens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/hitfile"
)

func (m *MainScreen) openFileRaw(path string) {
	ext := filepath.Ext(path)
	content, err := os.ReadFile(path)
	if err != nil {
		m.StatusBar = "Error opening " + filepath.Base(path)
		return
	}

	kind := document.KindGeneric
	if ext == ".hit" {
		kind = document.KindHit
	} else if path == envPathGlobal {
		kind = document.KindEnv
	} else if ext == ".md" {
		kind = document.KindMarkdown
	}

	doc := m.Store.GetOrCreate(path, kind, string(content), content)
	m.ActiveFile = path

	switch kind {
	case document.KindHit:
		m.ViewMode = ViewRunner
		doc.ViewMode = document.ViewRunner
		var h hitfile.HitFile
		if err := json.Unmarshal(content, &h); err == nil {
			doc.HitContent = &h
			resolved := hitfile.Resolve(&h, m.EnvData)
			m.URLBar.SetContent(resolved.Method, resolved.URL)
			m.Params.SetContent(resolved.Params)
			m.Headers.SetContent(resolved.Headers)
			m.Body.SetContent(resolved.Body)
			if h.Response != nil {
				respData, _ := json.Marshal(h.Response)
				var respMap map[string]interface{}
				if json.Unmarshal(respData, &respMap) == nil {
					status, statusText, durationMs, sizeBytes, ok := extractResponseFields(respMap)
					respBody, _ := json.MarshalIndent(respMap["data"], "", "  ")
					m.Response.SetResponse(status, statusText, durationMs, sizeBytes, ok, string(respBody))
				}
			}
		}
		m.Focus = FocusURLBar
		m.URLBar.Focus()
		m.ExplorerFocused = false

	case document.KindEnv:
		m.ViewMode = ViewText
		doc.ViewMode = document.ViewText
		env := make(map[string]string)
		json.Unmarshal(content, &env)
		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		envText := ""
		for _, k := range keys {
			envText += k + "=" + env[k] + "\n"
		}
		m.TextEd.SetContent(path, envText)
		m.Focus = FocusTextEditor
		m.TextEd.Focus()
		m.ExplorerFocused = false

	default:
		m.ViewMode = ViewText
		doc.ViewMode = document.ViewText
		m.TextEd.SetContent(path, string(content))
		m.Focus = FocusTextEditor
		m.TextEd.Focus()
		m.ExplorerFocused = false
	}
}

func extractResponseFields(respMap map[string]interface{}) (int, string, int64, int, bool) {
	status := 0
	statusText := ""
	var durationMs int64
	sizeBytes := 0
	ok := false
	if s, ok := respMap["status"].(float64); ok {
		status = int(s)
	}
	if s, ok := respMap["statusText"].(string); ok {
		statusText = s
	}
	if s, ok := respMap["durationMs"].(float64); ok {
		durationMs = int64(s)
	}
	if s, ok := respMap["sizeBytes"].(float64); ok {
		sizeBytes = int(s)
	}
	if s, ok := respMap["ok"].(bool); ok {
		ok = s
	}
	return status, statusText, durationMs, sizeBytes, ok
}
