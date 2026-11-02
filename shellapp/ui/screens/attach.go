package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/hittable/shellapp/internal/hitfile"
	"github.com/hittable/shellapp/ui/components/palette"
	"github.com/hittable/shellapp/ui/components/requesteditor"
	"github.com/hittable/shellapp/ui/theme"
)

// File uploads in the Body tab: the type line (alt+t), attaching a file
// (alt+a) and the list of referenced files. The body format is shared with
// the web app; see docs/file-uploads.md.

// Keys, with the composed runes macOS terminals send for ⌥t and ⌥a.
func isTypeKey(k string) bool   { return k == "alt+t" || k == "†" }
func isAttachKey(k string) bool { return k == "alt+a" || k == "å" }

// bodyContentType is the Content-Type of the request being edited.
func (m *MainScreen) bodyContentType() string {
	if h := m.currentHit(); h != nil {
		return hitfile.ContentType(h.Headers)
	}
	return ""
}

func (m *MainScreen) bodyType() requesteditor.BodyType {
	return m.Body.Type(m.bodyContentType())
}

// setBodyType switches the body to t: the Content-Type follows unless the
// user set a custom one, and a body that cannot be of that type is seeded
// (both edits are undoable in their editors).
func (m *MainScreen) setBodyType(t requesteditor.BodyType) {
	if h := m.currentHit(); h != nil && m.Headers.GetContent() != nil {
		if next, changed := requesteditor.HeadersForBodyType(h.Headers, t); changed {
			m.Headers.SetValue(marshalHeaders(next))
		}
	}
	body := m.Body.GetContent()
	switch t {
	case requesteditor.BodyMultipart:
		if _, err := hitfile.ParseFileRefs(body, hitfile.Multipart); err != nil || strings.TrimSpace(body) == "" {
			m.Body.SetValue("{}")
			if strings.TrimSpace(body) != "" {
				m.StatusBar = "Body reset to {} · ctrl+z brings the old one back"
			}
		}
	case requesteditor.BodyFile:
		if requesteditor.InferBodyType(body, "") != requesteditor.BodyFile {
			m.Body.SetValue("@")
			m.Body.MoveCursor(0, 1)
			if strings.TrimSpace(body) != "" {
				m.StatusBar = "Body replaced by @path · ctrl+z brings the old one back"
			}
		}
	}
	m.Body.Choose(t)
	if m.StatusBar == "" {
		m.StatusBar = "Body type: " + t.String()
	}
	m.saveAndEnqueueDebounced()
}

func (m *MainScreen) cycleBodyType() {
	m.setBodyType((m.bodyType() + 1) % requesteditor.BodyType(len(requesteditor.BodyTypeNames)))
}

// marshalHeaders formats headers like the Headers tab does, keeping <<KEY>>
// readable.
func marshalHeaders(h map[string]string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(h)
	return strings.TrimSuffix(b.String(), "\n")
}

// beginAttach picks a file for the body: the find palette in a project, the
// path prompt in single-file mode (or alt+a again from the palette).
func (m *MainScreen) beginAttach() {
	switch m.bodyType() {
	case requesteditor.BodyMultipart, requesteditor.BodyFile:
	default:
		if strings.TrimSpace(m.Body.GetContent()) != "" {
			m.StatusBar = "Attach needs a multipart or file body · alt+t switches the type"
			return
		}
	}
	if m.SingleFile {
		m.PathPrompt.Show(filepath.Dir(m.ActiveFile))
		return
	}
	m.openPalette(palette.ModeFiles)
	m.attaching = true
}

// attachFile puts the picked file into the body: a multipart field, or the
// whole body for a file (or empty) body.
func (m *MainScreen) attachFile(abs string) {
	m.attaching = false
	m.ActiveTab = requesteditor.TabBody
	m.Focus = FocusBody
	m.focusCurrent()
	ref := m.refPath(abs)
	if m.bodyType() == requesteditor.BodyMultipart {
		row, col := m.Body.Cursor()
		body := m.Body.GetContent()
		m.Body.SetValue(requesteditor.AttachToMultipart(body, requesteditor.Offset(body, row, col), ref))
	} else {
		if t := m.bodyType(); t != requesteditor.BodyFile {
			m.setBodyType(requesteditor.BodyFile)
		}
		m.Body.SetValue("@" + ref)
		m.Body.MoveCursor(0, len([]rune(ref))+1)
		m.Body.Choose(requesteditor.BodyFile)
	}
	m.StatusBar = "Attached " + ref
	m.saveAndEnqueueDebounced()
}

// refPath is how a picked file is written in the body: relative to the .hit
// file's folder when it is inside that folder or the project, else absolute.
func (m *MainScreen) refPath(abs string) string {
	dir := filepath.Dir(m.ActiveFile)
	inside := func(root string) bool {
		rel, err := filepath.Rel(root, abs)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if inside(dir) || !m.SingleFile && inside(m.RootDir) {
		if rel, err := filepath.Rel(dir, abs); err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(abs)
}

// renderBodyType is the row above the body editor: the type picker, or the
// path prompt while it is open.
func (m *MainScreen) renderBodyType(width int) string {
	if m.PathPrompt.Open {
		return m.PathPrompt.View(width)
	}
	cur := m.bodyType()
	parts := []string{theme.MutedStyle.Render(" type:")}
	for i, name := range requesteditor.BodyTypeNames {
		style := theme.TabInactiveStyle
		if requesteditor.BodyType(i) == cur {
			style = theme.TabActiveStyle
		} else if m.HoverZone == "btype_"+name {
			style = theme.HoverStyle
		}
		if i > 0 {
			parts = append(parts, theme.MutedStyle.Render("·"))
		}
		parts = append(parts, m.Zones.Mark("btype_"+name, style.Render(name)))
	}
	line := strings.Join(parts, " ")
	hint := "alt+t type"
	if cur == requesteditor.BodyMultipart || cur == requesteditor.BodyFile || strings.TrimSpace(m.Body.GetContent()) == "" {
		hint += " · alt+a attach"
	}
	if gap := width - lipgloss.Width(line) - lipgloss.Width(hint) - 1; gap >= 2 {
		line += strings.Repeat(" ", gap) + theme.MutedStyle.Render(hint)
	}
	if lipgloss.Width(line) > width {
		// Too narrow for every name: show the current one only.
		return ansi.Truncate(theme.MutedStyle.Render(" type: ")+theme.TabActiveStyle.Render(cur.String())+theme.MutedStyle.Render(" · alt+t"), width, "…")
	}
	return line
}

// bodyFileLines lists the files the body refers to, resolved the way a send
// would (variables from the current env, paths from the .hit file's folder):
// ✓ with the size, or ✗ when it cannot be read. At most max lines.
func (m *MainScreen) bodyFileLines(width, max int) []string {
	h := m.currentHit()
	if h == nil || max <= 0 {
		return nil
	}
	r := hitfile.Resolve(&hitfile.HitFile{Body: h.Body, Headers: h.Headers}, m.EnvData)
	refs, err := hitfile.ParseFileRefs(r.Body, hitfile.ContentType(r.Headers))
	if err != nil {
		return []string{ansi.Truncate(theme.StatusWarnStyle.Render(" ⚠ "+err.Error()), width, "…")}
	}
	dir := filepath.Dir(m.ActiveFile)
	var out []string
	files := refs.Files()
	for i, f := range files {
		if len(out) == max-1 && len(files) > max {
			out = append(out, theme.MutedStyle.Render(fmt.Sprintf(" … %d more", len(files)-i)))
			break
		}
		var line string
		fi, err := os.Stat(hitfile.ResolvePath(dir, f.Path))
		switch {
		case err != nil:
			line = theme.StatusErrorStyle.Render(" ✗ "+f.Path) + theme.MutedStyle.Render(" missing")
		case fi.IsDir():
			line = theme.StatusErrorStyle.Render(" ✗ "+f.Path) + theme.MutedStyle.Render(" is a folder")
		default:
			line = theme.StatusOKStyle.Render(" ✓ "+f.Path) + theme.MutedStyle.Render(" "+humanSize(int(fi.Size())))
		}
		out = append(out, ansi.Truncate(line, width, "…"))
	}
	return out
}
