package requesteditor

import (
	"encoding/json"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hittable/shellapp/ui/components/texteditor"
)

const jsonHint = `JSON object {"key": "value"}`

// jsonTab is a code editor holding a flat string→string JSON object
// (Params and Headers). Invalid JSON is kept in the buffer and flagged; the
// last valid value is what gets saved.
type jsonTab struct {
	*texteditor.TextEditor
	path        string
	InvalidJSON bool
}

func newJSONTab(name string) jsonTab {
	ed := texteditor.New()
	ed.SetContent(name, "{}")
	ed.Hint = jsonHint
	return jsonTab{TextEditor: ed, path: name}
}

func (j *jsonTab) SetContent(m map[string]string) {
	if m == nil {
		m = map[string]string{}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		data = []byte("{}")
	}
	j.TextEditor.RemoveEditor(j.path) // fresh undo history per file
	j.TextEditor.SetContent(j.path, string(data))
	j.validate()
}

// GetContent parses the buffer; nil means invalid JSON.
func (j *jsonTab) GetContent() map[string]string {
	raw := j.TextArea.Value()
	if strings.TrimSpace(raw) == "" {
		j.setValid(true)
		return map[string]string{}
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		j.setValid(false)
		return nil
	}
	if out == nil {
		out = map[string]string{}
	}
	j.setValid(true)
	return out
}

func (j *jsonTab) validate() {
	raw := j.TextArea.Value()
	if strings.TrimSpace(raw) == "" {
		j.setValid(true)
		return
	}
	var m map[string]string
	j.setValid(json.Unmarshal([]byte(raw), &m) == nil)
}

func (j *jsonTab) setValid(ok bool) {
	j.InvalidJSON = !ok
	if ok {
		j.Hint = jsonHint
	} else {
		j.Hint = "⚠ invalid JSON — last valid value is kept"
	}
}

func (j *jsonTab) Update(msg tea.Msg) tea.Cmd {
	cmd := j.TextEditor.Update(msg)
	j.validate()
	return cmd
}

type ParamsTab struct{ jsonTab }
type HeadersTab struct{ jsonTab }

func NewParamsTab() *ParamsTab   { return &ParamsTab{newJSONTab("params.json")} }
func NewHeadersTab() *HeadersTab { return &HeadersTab{newJSONTab("headers.json")} }
