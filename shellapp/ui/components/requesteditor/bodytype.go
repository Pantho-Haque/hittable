package requesteditor

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/hittable/shellapp/internal/hitfile"
)

// BodyType is how the body is sent. It is never stored: like the web app
// (utils/bodyType.ts) it is inferred from the body and its Content-Type, so
// .hit files stay exactly as they were.
type BodyType int

const (
	BodyJSON BodyType = iota
	BodyText
	BodyForm
	BodyMultipart
	BodyFile
)

var BodyTypeNames = []string{"json", "text", "form", "multipart", "file"}

func (t BodyType) String() string { return BodyTypeNames[t] }

// bodyTypeCT is the Content-Type each type sets (file keeps the user's, or
// sends one guessed from the file name).
var bodyTypeCT = []string{"application/json", "text/plain", "application/x-www-form-urlencoded", hitfile.Multipart, ""}

var jsonCT = regexp.MustCompile(`[/+]json\b`)

// InferBodyType mirrors the web app's inferBodyType.
func InferBodyType(body, contentType string) BodyType {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "x-www-form-urlencoded") {
		return BodyForm
	}
	if strings.HasPrefix(ct, hitfile.Multipart) {
		return BodyMultipart
	}
	// `@path` on one line is a file whatever the content type; `@;…` is one
	// still being typed.
	if refs, err := hitfile.ParseFileRefs(body, ""); err != nil || refs != nil && refs.Kind == "binary" {
		return BodyFile
	}
	if jsonCT.MatchString(ct) {
		return BodyJSON
	}
	if strings.HasPrefix(ct, "text/") {
		return BodyText
	}
	t := strings.TrimSpace(body)
	if t == "" {
		if ct != "" {
			return BodyText
		}
		return BodyJSON
	}
	if strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") || json.Valid([]byte(t)) {
		return BodyJSON
	}
	return BodyText
}

// HeadersForBodyType mirrors the web app's headersForBodyType: the type's
// Content-Type replaces one this app sets itself (or fills a missing one);
// a custom one the user typed wins. Returns the headers and whether they
// changed.
func HeadersForBodyType(h map[string]string, t BodyType) (map[string]string, bool) {
	key := hitfile.HeaderKey(h, "Content-Type")
	cur := strings.ToLower(h[key])
	ours := key == ""
	for _, ct := range bodyTypeCT {
		if ct != "" && strings.HasPrefix(cur, ct) {
			ours = true
		}
	}
	if !ours {
		return h, false
	}
	next := make(map[string]string, len(h)+1)
	for k, v := range h {
		if k != key {
			next[k] = v
		}
	}
	if ct := bodyTypeCT[t]; ct != "" {
		if key == "" {
			key = "Content-Type"
		}
		next[key] = ct
	}
	return next, len(next) != len(h) || next[key] != h[key]
}

// Offset is the byte offset of a line / rune column in s.
func Offset(s string, row, col int) int {
	off := 0
	for i, line := range strings.Split(s, "\n") {
		if i == row {
			r := []rune(line)
			return off + len(string(r[:min(col, len(r))]))
		}
		off += len(line) + 1
	}
	return len(s)
}

// AttachToMultipart puts "@ref" into a multipart body: as the value of the
// JSON string the cursor (byte offset off) is in, or else as a new "file"
// member at the end of the object ("file2"… when taken).
func AttachToMultipart(body string, off int, ref string) string {
	val := jsonString("@" + ref)
	// Find the string literal around the cursor.
	inStr, esc, start := false, false, -1
	for i := 0; i < len(body); i++ {
		c := body[i]
		if !inStr {
			if c == '"' {
				inStr, start = true, i
			}
			continue
		}
		switch {
		case esc:
			esc = false
		case c == '\\':
			esc = true
		case c == '"':
			inStr = false
			if start < off && off <= i {
				// A value when the string follows a colon.
				if before := strings.TrimRight(body[:start], " \t\r\n"); strings.HasSuffix(before, ":") {
					return body[:start] + val + body[i+1:]
				}
			}
		}
	}

	name := "file"
	if refs, err := hitfile.ParseFileRefs(body, hitfile.Multipart); err == nil {
		taken := map[string]bool{}
		for _, p := range refs.Parts {
			taken[p.Name] = true
		}
		for n := 2; taken[name]; n++ {
			name = "file" + strconv.Itoa(n)
		}
	}
	member := jsonString(name) + ": " + val
	end := strings.LastIndex(body, "}")
	if end < 0 || !strings.HasPrefix(strings.TrimSpace(body), "{") {
		return "{\n  " + member + "\n}"
	}
	before := strings.TrimRight(body[:end], " \t\r\n")
	sep := ","
	if strings.HasSuffix(before, "{") {
		sep = ""
	}
	return before + sep + "\n  " + member + "\n" + body[end:]
}

func jsonString(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}
