package requesteditor

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/hittable/shellapp/ui/components/texteditor"
)

// BodyTab is a code editor for the raw request body.
type BodyTab struct {
	*texteditor.TextEditor
}

const bodyPath = "body.json"

func NewBodyTab() *BodyTab {
	ed := texteditor.New()
	ed.SetContent(bodyPath, "")
	ed.Hint = "raw body · ctrl+l formats JSON"
	return &BodyTab{TextEditor: ed}
}

func (b *BodyTab) SetContent(body string) {
	b.TextEditor.RemoveEditor(bodyPath)
	b.TextEditor.SetContent(bodyPath, body)
	b.updateHint()
}

func (b *BodyTab) GetContent() string { return b.TextArea.Value() }

func (b *BodyTab) updateHint() {
	v := b.TextArea.Value()
	if strings.TrimSpace(v) != "" && !json.Valid([]byte(v)) {
		b.Hint = "raw body (not JSON)"
	} else {
		b.Hint = "raw body · ctrl+l formats JSON"
	}
}

// FormatJSON pretty-prints the body if it is valid JSON.
func (b *BodyTab) FormatJSON() bool {
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(b.TextArea.Value()), "", "  "); err != nil {
		return false
	}
	b.SetValue(out.String())
	return true
}
