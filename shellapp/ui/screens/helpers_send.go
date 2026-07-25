package screens

import (
	"encoding/json"

	"github.com/hittable/shellapp/internal/document"
	"github.com/hittable/shellapp/internal/hitfile"
)

func (m *MainScreen) sendRequestAsync() {
	if m.ActiveFile == "" {
		return
	}
	doc := m.Store.Get(m.ActiveFile)
	if doc == nil || doc.Kind != document.KindHit {
		return
	}
	m.Sending = true

	method, _ := m.URLBar.GetContent()
	url := m.URLBar.URLInput.Value()
	hdrs := m.Headers.GetContent()
	params := m.Params.GetContent()
	body := m.Body.GetContent()
	envSnap := make(map[string]string, len(m.EnvData))
	for k, v := range m.EnvData {
		envSnap[k] = v
	}

	go func() {
		url = interpolateString(url, envSnap)
		for k, v := range hdrs {
			hdrs[k] = interpolateString(v, envSnap)
		}
		for k, v := range params {
			params[k] = interpolateString(v, envSnap)
		}
		body = interpolateString(body, envSnap)

		result, err := hitfile.ExecuteAndCapture(&hitfile.HitFile{
			Method: method, URL: url, Headers: hdrs, Params: params, Body: body,
		}, envSnap)

		teaProgram.Send(responseMsg{result: result, err: err})
	}()
}

func (m *MainScreen) handleResponseMsg(msg responseMsg) {
	m.Sending = false
	if msg.err != nil {
		m.StatusBar = msg.err.Error()
		return
	}
	if msg.result != nil {
		respBody, _ := json.MarshalIndent(msg.result.Data, "", "  ")
		m.Response.SetResponse(
			msg.result.Status, msg.result.StatusText,
			msg.result.DurationMs, msg.result.SizeBytes,
			msg.result.Ok, string(respBody),
		)
		doc := m.Store.Get(m.ActiveFile)
		if doc != nil && doc.Kind == document.KindHit {
			doc.Lock()
			if h, ok := doc.HitContent.(*hitfile.HitFile); ok {
				h.Response = &hitfile.Response{
					Data: msg.result.Data, Status: msg.result.Status,
					StatusText: msg.result.StatusText, Ok: msg.result.Ok,
					Headers: msg.result.Headers, Cookies: msg.result.Cookies,
					DurationMs: msg.result.DurationMs, SizeBytes: msg.result.SizeBytes,
				}
			}
			doc.Unlock()
			m.saveAndEnqueue()
		}
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
		m.ViewMode = ViewText
		doc.ViewMode = document.ViewText
		method, _ := m.URLBar.GetContent()
		url := m.URLBar.URLInput.Value()
		headers := m.Headers.GetContent()
		params := m.Params.GetContent()
		body := m.Body.GetContent()
		h := &hitfile.HitFile{
			Method: method, URL: url, Headers: headers, Params: params, Body: body,
		}
		if doc.HitContent != nil {
			if existing, ok := doc.HitContent.(*hitfile.HitFile); ok && existing.Response != nil {
				h.Response = existing.Response
			}
		}
		data, _ := json.MarshalIndent(h, "", "  ")
		m.TextEd.SetContent(m.ActiveFile, string(data)+"\n")
		m.Focus = FocusTextEditor
		m.TextEd.Focus()
		m.URLBar.Blur()
		m.Params.Blur()
		m.Headers.Blur()
		m.Body.Blur()
	} else {
		content := m.TextEd.GetContent()
		var h hitfile.HitFile
		if err := json.Unmarshal([]byte(content), &h); err != nil {
			m.StatusBar = "Invalid JSON — fix errors before switching to Runner"
			return
		}
		m.ViewMode = ViewRunner
		doc.ViewMode = document.ViewRunner
		doc.HitContent = &h
		resolved := hitfile.Resolve(&h, m.EnvData)
		m.URLBar.SetContent(resolved.Method, resolved.URL)
		m.Params.SetContent(resolved.Params)
		m.Headers.SetContent(resolved.Headers)
		m.Body.SetContent(resolved.Body)
		m.Focus = FocusURLBar
		m.URLBar.Focus()
		m.TextEd.Blur()
	}
}
