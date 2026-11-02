package hitfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// File uploads in a .hit body. The rules are shared with the web app
// (utils/fileBody.ts); see docs/file-uploads.md and
// testdata/filebody_cases.json.

const Multipart = "multipart/form-data"

// FileSpec is `path[;type=T][;filename=F]`.
type FileSpec struct {
	Path, Type, Filename string
}

// FilePart is one multipart field: a text Value, or a File when File != nil.
type FilePart struct {
	Name  string
	Value string
	File  *FileSpec
}

// FileRefs is what ParseFileRefs found: Kind is "raw" (Body is sent), "binary"
// (File's bytes are the body) or "multipart" (Parts, in document order).
type FileRefs struct {
	Kind  string
	Body  string
	File  *FileSpec
	Parts []FilePart
}

// Files lists every file the body refers to.
func (r *FileRefs) Files() []FileSpec {
	switch {
	case r == nil:
		return nil
	case r.Kind == "binary":
		return []FileSpec{*r.File}
	}
	var out []FileSpec
	for _, p := range r.Parts {
		if p.File != nil {
			out = append(out, *p.File)
		}
	}
	return out
}

// IsMultipart reports whether a Content-Type value is multipart/form-data.
func IsMultipart(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), Multipart)
}

// HeaderKey finds a header name case-insensitively ("" when absent). With
// several spellings present the alphabetically first wins, so the choice is
// stable.
func HeaderKey(h map[string]string, name string) string {
	var keys []string
	for k := range h {
		if strings.EqualFold(k, name) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

// ContentType is the request's Content-Type value, or "".
func ContentType(h map[string]string) string {
	if k := HeaderKey(h, "Content-Type"); k != "" {
		return h[k]
	}
	return ""
}

var errMultipartBody = errors.New("a multipart body is a JSON object of fields")

// ParseFileRefs finds the file references in a body. nil, nil means none:
// the body is sent as it is.
func ParseFileRefs(body, contentType string) (*FileRefs, error) {
	if IsMultipart(contentType) {
		if strings.TrimSpace(body) == "" {
			return &FileRefs{Kind: "multipart", Parts: []FilePart{}}, nil
		}
		members, err := objectMembers(body)
		if err != nil {
			return nil, err
		}
		parts := make([]FilePart, 0, len(members))
		for _, mb := range members {
			p := FilePart{Name: mb.name}
			var s string
			switch {
			case json.Unmarshal(mb.raw, &s) != nil: // not a string
				if string(mb.raw) != "null" {
					var buf bytes.Buffer
					_ = json.Compact(&buf, mb.raw)
					p.Value = buf.String()
				}
			case strings.HasPrefix(s, "@@"):
				p.Value = s[1:]
			case strings.HasPrefix(s, "@"):
				f, err := parseFileSpec(s[1:])
				if err != nil {
					return nil, fmt.Errorf("%s: %w", mb.name, err)
				}
				p.File = &f
			default:
				p.Value = s
			}
			parts = append(parts, p)
		}
		return &FileRefs{Kind: "multipart", Parts: parts}, nil
	}
	t := strings.TrimSpace(body)
	if len(t) < 2 || !strings.HasPrefix(t, "@") || strings.Contains(t, "\n") {
		return nil, nil
	}
	if strings.HasPrefix(t, "@@") {
		return &FileRefs{Kind: "raw", Body: t[1:]}, nil
	}
	f, err := parseFileSpec(t[1:])
	if err != nil {
		return nil, err
	}
	return &FileRefs{Kind: "binary", File: &f}, nil
}

// parseFileSpec splits trailing ;type= / ;filename= options off a path. Only
// the last occurrence of each counts; any other ';' stays in the path.
func parseFileSpec(spec string) (FileSpec, error) {
	var f FileSpec
	var haveType, haveName bool
	for {
		i := strings.LastIndex(spec, ";")
		if i < 0 {
			break
		}
		seg := spec[i+1:]
		if v, ok := strings.CutPrefix(seg, "type="); ok {
			if !haveType {
				f.Type, haveType = v, true
			}
		} else if v, ok := strings.CutPrefix(seg, "filename="); ok {
			if !haveName {
				f.Filename, haveName = v, true
			}
		} else {
			break
		}
		spec = spec[:i]
	}
	f.Path = strings.TrimSpace(spec)
	if f.Path == "" {
		return f, errors.New("a file field needs a path after @")
	}
	return f, nil
}

type member struct {
	name string
	raw  json.RawMessage
}

// objectMembers reads a JSON object's members in the order JavaScript's
// Object.entries(JSON.parse(s)) gives them, so both apps send the same parts:
// a repeated key keeps its first position with its last value, and keys that
// are array indices ("0", "7") come first in numeric order.
func objectMembers(s string) ([]member, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, errMultipartBody
	}
	var out []member
	at := map[string]int{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, errMultipartBody
		}
		name, _ := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, errMultipartBody
		}
		if i, ok := at[name]; ok {
			out[i].raw = raw
			continue
		}
		at[name] = len(out)
		out = append(out, member{name, raw})
	}
	if _, err := dec.Token(); err != nil { // the closing }
		return nil, errMultipartBody
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errMultipartBody
	}
	index := func(k string) (uint64, bool) {
		n, err := strconv.ParseUint(k, 10, 32)
		return n, err == nil && n < 1<<32-1 && strconv.FormatUint(n, 10) == k
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, aok := index(out[i].name)
		b, bok := index(out[j].name)
		if aok && bok {
			return a < b
		}
		return aok && !bok
	})
	return out, nil
}

// ResolvePath turns a path from a body into a file on disk: relative paths
// are taken from baseDir (the .hit file's folder); absolute paths and ~/ are
// used as they are. `\` is read as `/`.
func ResolvePath(baseDir, p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, filepath.FromSlash(p[1:]))
		}
	}
	if path.IsAbs(p) || filepath.IsAbs(filepath.FromSlash(p)) {
		return filepath.Clean(filepath.FromSlash(p))
	}
	return filepath.Join(baseDir, filepath.FromSlash(p))
}

// FileName is the name a file part is uploaded as.
func (f FileSpec) FileName() string {
	if f.Filename != "" {
		return f.Filename
	}
	return path.Base(strings.ReplaceAll(f.Path, `\`, "/"))
}

// MimeType is the file's type option, else a guess from its extension, else
// application/octet-stream.
func (f FileSpec) MimeType() string {
	if f.Type != "" {
		return f.Type
	}
	if t := mimeTypes[strings.ToLower(strings.TrimPrefix(path.Ext(f.FileName()), "."))]; t != "" {
		return t
	}
	return "application/octet-stream"
}

// mimeTypes is the web app's table (guessType in utils/fileBody.ts), so both
// apps send the same type on every OS.
var mimeTypes = map[string]string{
	"png": "image/png", "jpg": "image/jpeg", "jpeg": "image/jpeg", "gif": "image/gif", "webp": "image/webp", "svg": "image/svg+xml",
	"avif": "image/avif", "ico": "image/x-icon", "bmp": "image/bmp", "pdf": "application/pdf", "json": "application/json",
	"txt": "text/plain", "csv": "text/csv", "html": "text/html", "htm": "text/html", "xml": "application/xml", "md": "text/markdown",
	"zip": "application/zip", "gz": "application/gzip", "tar": "application/x-tar", "mp3": "audio/mpeg", "wav": "audio/wav",
	"mp4": "video/mp4", "webm": "video/webm", "mov": "video/quicktime", "js": "text/javascript", "css": "text/css",
	"yaml": "application/yaml", "yml": "application/yaml", "doc": "application/msword",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
}
