package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/hitfile"
)

// reloadEnv picks up external edits to env.json unless it is open in-app
// (then EnvData is already the freshest copy).
func (m *MainScreen) reloadEnv() {
	if m.Store.Get(m.EnvPath) != nil {
		return
	}
	data, err := os.ReadFile(m.EnvPath)
	if err != nil {
		return
	}
	var env map[string]string
	if json.Unmarshal(data, &env) == nil && env != nil {
		m.EnvData = env
	}
}

func (m *MainScreen) sendRequestAsync() tea.Cmd {
	h := m.currentHit()
	if h == nil {
		return nil
	}
	if strings.TrimSpace(h.URL) == "" {
		m.StatusBar = "URL is empty"
		return nil
	}
	if m.Sending || teaProgram == nil {
		return nil
	}
	m.reloadEnv()
	m.Sending = true
	path := m.ActiveFile
	envSnap := make(map[string]string, len(m.EnvData))
	for k, v := range m.EnvData {
		envSnap[k] = v
	}
	req := *h
	req.Response = nil

	go func() {
		result, err := hitfile.ExecuteAndCapture(&req, envSnap)
		teaProgram.Send(responseMsg{path: path, result: result, err: err})
	}()
	return m.Spinner.Tick
}

// bodyText renders a response payload for display: JSON is indented,
// plain strings (HTML, text) are shown verbatim.
func bodyText(data interface{}) string {
	if s, ok := data.(string); ok {
		return s
	}
	if data == nil {
		return ""
	}
	b, _ := json.MarshalIndent(data, "", "  ")
	return string(b)
}

func (m *MainScreen) handleResponseMsg(msg responseMsg) {
	m.Sending = false
	if msg.err != nil {
		m.StatusBar = "Request failed: " + msg.err.Error()
		return
	}
	if msg.result == nil || msg.path != m.ActiveFile {
		return
	}
	m.Response.SetResponse(
		msg.result.Status, msg.result.StatusText,
		msg.result.DurationMs, msg.result.SizeBytes,
		msg.result.Ok, bodyText(msg.result.Data),
	)
	m.Response.SetHeaders(msg.result.Headers)
	m.StatusBar = fmt.Sprintf("%d %s in %dms", msg.result.Status, msg.result.StatusText, msg.result.DurationMs)
	doc := m.Store.Get(m.ActiveFile)
	if doc == nil || doc.Kind != document.KindHit {
		return
	}
	if h, ok := doc.HitContent.(*hitfile.HitFile); ok {
		h.Response = msg.result
		if m.ViewMode == ViewText {
			m.TextEd.SetContent(m.ActiveFile, string(hitfile.Marshal(h)))
		}
	}
	m.saveAndEnqueue()
}

// loadHitIntoRunner fills the Runner widgets from h (raw values; <<KEY>>
// templates are resolved only at send time).
func (m *MainScreen) loadHitIntoRunner(h *hitfile.HitFile) {
	m.URLBar.SetContent(h.Method, h.URL)
	m.Params.SetContent(h.Params)
	m.Headers.SetContent(h.Headers)
	m.Body.SetContent(h.Body)
	if h.Response != nil {
		m.Response.SetResponse(h.Response.Status, h.Response.StatusText,
			h.Response.DurationMs, h.Response.SizeBytes, h.Response.Ok, bodyText(h.Response.Data))
		m.Response.SetHeaders(h.Response.Headers)
	} else {
		m.Response.SetResponse(0, "", 0, 0, false, "")
	}
}

func (m *MainScreen) toggleViewMode() {
	if m.ActiveFile == "" {
		return
	}
	doc := m.Store.Get(m.ActiveFile)
	if doc == nil || doc.Kind != document.KindHit {
		return
	}
	if m.ViewMode == ViewRunner {
		h := m.currentHit()
		if h == nil {
			return
		}
		doc.HitContent = h
		m.TextEd.SetContent(m.ActiveFile, string(hitfile.Marshal(h)))
		m.ViewMode = ViewText
		doc.ViewMode = document.ViewText
		m.Focus = FocusTextEditor
		m.focusCurrent()
	} else {
		var h hitfile.HitFile
		if err := json.Unmarshal([]byte(m.TextEd.GetContent()), &h); err != nil {
			m.StatusBar = "Invalid JSON — fix errors before switching to Runner"
			return
		}
		doc.HitContent = &h
		m.loadHitIntoRunner(&h)
		m.ViewMode = ViewRunner
		doc.ViewMode = document.ViewRunner
		m.Focus = FocusURLBar
		m.focusCurrent()
	}
	m.saveAndEnqueue()
}
