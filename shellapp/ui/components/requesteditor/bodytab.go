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
	// A type picked with alt+t holds while the body is unchanged, even when
	// inference would say otherwise (an empty body switched to file is "@").
	chosen    BodyType
	chosenFor *string
}

// Type is the body's type given the request's Content-Type.
func (b *BodyTab) Type(contentType string) BodyType {
	v := b.GetContent()
	if b.chosenFor != nil && *b.chosenFor == v {
		return b.chosen
	}
	return InferBodyType(v, contentType)
}

// Choose pins t as the type until the body is next edited.
func (b *BodyTab) Choose(t BodyType) {
	v := b.GetContent()
	b.chosen, b.chosenFor = t, &v
}

const bodyPath = "body.json"

func NewBodyTab() *BodyTab {
	ed := texteditor.New()
	ed.SetContent(bodyPath, "")
	ed.Hint = "ctrl+l formats JSON"
	return &BodyTab{TextEditor: ed}
}

func (b *BodyTab) SetContent(body string) {
	b.TextEditor.RemoveEditor(bodyPath)
	b.TextEditor.SetContent(bodyPath, body)
	b.chosenFor = nil
	b.updateHint()
}

func (b *BodyTab) GetContent() string { return b.TextArea.Value() }

func (b *BodyTab) updateHint() {
	v := b.TextArea.Value()
	if strings.TrimSpace(v) != "" && !json.Valid([]byte(v)) {
		b.Hint = "raw body (not JSON)"
	} else {
		b.Hint = "ctrl+l formats JSON"
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
