package hitfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// shape renders a result in the cases file's JSON shape.
func shape(r *FileRefs) any {
	if r == nil {
		return nil
	}
	file := func(f *FileSpec) map[string]any {
		m := map[string]any{"path": f.Path}
		if f.Type != "" {
			m["type"] = f.Type
		}
		if f.Filename != "" {
			m["filename"] = f.Filename
		}
		return m
	}
	switch r.Kind {
	case "raw":
		return map[string]any{"kind": "raw", "body": r.Body}
	case "binary":
		return map[string]any{"kind": "binary", "file": file(r.File)}
	}
	parts := []any{}
	for _, p := range r.Parts {
		if p.File != nil {
			parts = append(parts, map[string]any{"name": p.Name, "file": file(p.File)})
		} else {
			parts = append(parts, map[string]any{"name": p.Name, "value": p.Value})
		}
	}
	return map[string]any{"kind": "multipart", "parts": parts}
}

// The golden table shared with the web app (identical copy in tests/unit/fixtures).
func TestParseFileRefsGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/filebody_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name        string `json:"name"`
		ContentType string `json:"contentType"`
		Body        string `json:"body"`
		Expect      any    `json:"expect"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases")
	}
	for _, c := range cases {
		got, err := ParseFileRefs(c.Body, c.ContentType)
		if m, ok := c.Expect.(map[string]any); ok && m["error"] == true {
			if err == nil {
				t.Errorf("%s: want an error, got %#v", c.Name, shape(got))
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		// Round-trip through JSON so both sides use the same types.
		var g any
		b, _ := json.Marshal(shape(got))
		json.Unmarshal(b, &g)
		if !reflect.DeepEqual(g, c.Expect) {
			t.Errorf("%s:\n got %s\nwant %v", c.Name, b, c.Expect)
		}
	}
}

// Member order follows JavaScript's Object.entries(JSON.parse(...)), which the
// web app uses: a repeated key keeps its first slot and last value, and
// array-index keys come first.
func TestParseFileRefsMatchesJSObjectOrder(t *testing.T) {
	r, err := ParseFileRefs(`{"b":"1","10":"x","a":"2","b":"3","2":"y","01":"z"}`, "multipart/form-data")
	if err != nil {
		t.Fatal(err)
	}
	var names, values []string
	for _, p := range r.Parts {
		names, values = append(names, p.Name), append(values, p.Value)
	}
	if want := []string{"2", "10", "b", "a", "01"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names %v, want %v", names, want)
	}
	if values[2] != "3" {
		t.Errorf("repeated key keeps the last value, got %q", values[2])
	}
	if _, err := ParseFileRefs(`{"a":1} x`, "multipart/form-data"); err == nil {
		t.Error("trailing text after the object must be an error")
	}
	// A repeated option: the last one counts and the earlier one is dropped (as on the web).
	r, _ = ParseFileRefs("@a.txt;type=x;type=y", "")
	if r.File.Path != "a.txt" || r.File.Type != "y" {
		t.Errorf("repeated option: %+v", r.File)
	}
}

func TestResolvePath(t *testing.T) {
	home, _ := os.UserHomeDir()
	base := filepath.FromSlash("/proj/hittable/users")
	cases := []struct{ in, want string }{
		{"files/a.png", "/proj/hittable/users/files/a.png"},
		{"./a.png", "/proj/hittable/users/a.png"},
		{"../shared/a.png", "/proj/hittable/shared/a.png"},
		{"../../../../a.png", "/a.png"},
		{`files\win\a.png`, "/proj/hittable/users/files/win/a.png"},
		{"/abs/a.png", "/abs/a.png"},
		{"~/Pictures/a.png", filepath.ToSlash(filepath.Join(home, "Pictures/a.png"))},
	}
	for _, c := range cases {
		if got := filepath.ToSlash(ResolvePath(base, c.in)); got != c.want {
			t.Errorf("ResolvePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFileSpecNameAndType(t *testing.T) {
	cases := []struct {
		f          FileSpec
		name, kind string
	}{
		{FileSpec{Path: "files/a.png"}, "a.png", "image/png"},
		{FileSpec{Path: `dir\b.json`}, "b.json", "application/json"},
		{FileSpec{Path: "x/blob"}, "blob", "application/octet-stream"},
		{FileSpec{Path: "a.png", Type: "image/x-mine", Filename: "me.jpg"}, "me.jpg", "image/x-mine"},
		{FileSpec{Path: "a.bin", Filename: "me.png"}, "me.png", "image/png"},
	}
	for _, c := range cases {
		if c.f.FileName() != c.name || c.f.MimeType() != c.kind {
			t.Errorf("%+v: name %q type %q", c.f, c.f.FileName(), c.f.MimeType())
		}
	}
}
