package collection

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hittable/shellapp/internal/hitfile"
)

// fixture is a hittable/ tree a user might have built by hand.
var fixture = map[string]*hitfile.HitFile{
	"users/List users.hit": {Method: "GET", URL: "<<BASE>>/users",
		Headers: map[string]string{"Accept": "application/json", "Authorization": "Bearer <<TOKEN>>"},
		Params:  map[string]string{"page": "2", "q": "a b&c=d", "filter": "<<FILTER>>"}},
	"users/admin/Create user.hit": {Method: "POST", URL: "<<BASE>>/users",
		Headers: map[string]string{"Content-Type": "application/json"},
		Params:  map[string]string{},
		Body:    "{\n  \"name\": \"<<NAME>>\",\n  \"tags\": [\"a\", \"b\"]\n}"},
	"Health.hit": {Method: "HEAD", URL: "https://status.example.test/health",
		Headers: map[string]string{}, Params: map[string]string{}},
	"Delete thing.hit": {Method: "DELETE", URL: "<<BASE>>/things/42",
		Headers: map[string]string{"X-Reason": "cleanup"}, Params: map[string]string{"force": "true"}},
}

var fixtureEnv = map[string]string{"BASE": "https://api.example.test", "TOKEN": "t0k", "FILTER": "active", "NAME": "ann"}

func writeFixture(t *testing.T, hd string) {
	t.Helper()
	for rel, h := range fixture {
		p := filepath.Join(hd, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, hitfile.Marshal(h), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := json.Marshal(fixtureEnv)
	os.WriteFile(filepath.Join(hd, "env.json"), b, 0o644)
	// Things export must skip: notes, a broken .hit, an empty folder.
	os.MkdirAll(filepath.Join(hd, "notes"), 0o755)
	os.WriteFile(filepath.Join(hd, "notes", "readme.md"), []byte("# hi\n"), 0o644)
	os.WriteFile(filepath.Join(hd, "broken.hit"), []byte("{not json"), 0o644)
	os.MkdirAll(filepath.Join(hd, "empty"), 0o755)
}

// loadTree reads every .hit under dir, keyed by slash path relative to dir.
func loadTree(t *testing.T, dir string) map[string]*hitfile.HitFile {
	t.Helper()
	out := map[string]*hitfile.HitFile{}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".hit" {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = readHit(t, p)
		return nil
	})
	return out
}

func sameRequests(t *testing.T, label string, got, want map[string]*hitfile.HitFile) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: %d requests, want %d: %v", label, len(got), len(want), keys(got))
	}
	for rel, w := range want {
		g, ok := got[rel]
		if !ok {
			t.Errorf("%s: missing %s (have %v)", label, rel, keys(got))
			continue
		}
		if g.Method != w.Method || g.URL != w.URL || g.Body != w.Body ||
			!reflect.DeepEqual(g.Headers, w.Headers) || !reflect.DeepEqual(g.Params, w.Params) {
			t.Errorf("%s: %s differs\n got  %s %s h=%v p=%v b=%q\n want %s %s h=%v p=%v b=%q",
				label, rel, g.Method, g.URL, g.Headers, g.Params, g.Body, w.Method, w.URL, w.Headers, w.Params, w.Body)
		}
		if g.Response != nil {
			t.Errorf("%s: %s imported with a response", label, rel)
		}
	}
}

func keys[V any](m map[string]V) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func exportImport(t *testing.T, srcHD string, export func(string, string) ([]byte, int, error), name string) (*Report, string) {
	t.Helper()
	data, n, err := export(srcHD, name)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(fixture) {
		t.Errorf("exported %d requests, want %d (broken.hit and notes must be skipped)", n, len(fixture))
	}
	if !json.Valid(data) {
		t.Fatal("export is not valid JSON")
	}
	tmp := t.TempDir()
	file := filepath.Join(tmp, "export.json")
	os.WriteFile(file, data, 0o644)
	dst := filepath.Join(tmp, "hittable")
	rep, err := Import(file, dst)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	return rep, dst
}

func TestPostmanRoundTrip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "hittable")
	writeFixture(t, src)
	rep, dst := exportImport(t, src, ExportPostman, "My API")
	if rep.Format != "Postman v2.1" || rep.Name != "My API" || rep.Requests != len(fixture) || rep.Folders != 2 {
		t.Errorf("report = %+v", rep)
	}
	sameRequests(t, "postman", loadTree(t, rep.Dir), fixture)
	env := readEnvFile(t, dst)
	if !reflect.DeepEqual(env, fixtureEnv) {
		t.Errorf("env = %v, want %v", env, fixtureEnv)
	}
}

func TestInsomniaRoundTrip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "hittable")
	writeFixture(t, src)
	rep, dst := exportImport(t, src, ExportInsomnia, "My API")
	if rep.Format != "Insomnia" || rep.Name != "My API" || rep.Requests != len(fixture) || rep.Folders != 2 {
		t.Errorf("report = %+v", rep)
	}
	sameRequests(t, "insomnia", loadTree(t, rep.Dir), fixture)
	if env := readEnvFile(t, dst); !reflect.DeepEqual(env, fixtureEnv) {
		t.Errorf("env = %v", env)
	}
}

// Postman → hittable → Insomnia → hittable keeps everything.
func TestCrossFormatRoundTrip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "hittable")
	writeFixture(t, src)
	rep, _ := exportImport(t, src, ExportPostman, "X")
	data, _, err := ExportInsomnia(rep.Dir, "X")
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "i.json"), data, 0o644)
	rep2, err := Import(filepath.Join(tmp, "i.json"), filepath.Join(tmp, "hittable"))
	if err != nil {
		t.Fatal(err)
	}
	sameRequests(t, "postman→insomnia", loadTree(t, rep2.Dir), fixture)
}

func readEnvFile(t *testing.T, hd string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(hd, "env.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("env.json: %v\n%s", err, b)
	}
	return m
}

func TestExportEmptyAndMissing(t *testing.T) {
	hd := filepath.Join(t.TempDir(), "hittable")
	os.MkdirAll(hd, 0o755)
	for _, ex := range []func(string, string) ([]byte, int, error){ExportPostman, ExportInsomnia} {
		data, n, err := ex(hd, "empty")
		if err != nil || n != 0 || !json.Valid(data) {
			t.Errorf("empty export: n=%d err=%v valid=%v", n, err, json.Valid(data))
		}
		if _, _, err := ex(filepath.Join(hd, "nope"), "x"); err == nil {
			t.Error("export of a missing directory should fail")
		}
	}
}

func TestImportRejectsBadInput(t *testing.T) {
	tmp := t.TempDir()
	hd := filepath.Join(tmp, "hittable")
	cases := map[string]string{
		"empty":            "",
		"truncated":        `{"info":{"schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":[`,
		"null":             "null",
		"array of numbers": "[1,2,3]",
		"plain object":     `{"hello":"world"}`,
		"binary":           "\x00\x01\x02\xff\xfe",
		"postman bad item": `{"info":{"schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":"nope"}`,
		"insomnia bad res": `{"_type":"export","__export_format":4,"resources":"nope"}`,
		"html":             "<html><body>not json</body></html>",
	}
	for name, body := range cases {
		p := filepath.Join(tmp, strings.ReplaceAll(name, " ", "_")+".json")
		os.WriteFile(p, []byte(body), 0o644)
		if rep, err := Import(p, hd); err == nil {
			t.Errorf("%s: imported %+v, want an error", name, rep)
		}
	}
	if _, err := Import(filepath.Join(tmp, "missing.json"), hd); err == nil {
		t.Error("missing file should fail")
	}
	if _, err := os.Stat(hd); !os.IsNotExist(err) {
		t.Error("a rejected import must not create hittable/")
	}
}

// Malformed-but-parseable collections must import what they can, not panic.
func TestImportToleratesOddShapes(t *testing.T) {
	cases := map[string]string{
		"postman url number":   `{"info":{"schema":"getpostman.com"},"item":[{"name":"x","request":{"method":"get","url":123}}]}`,
		"postman null item":    `{"info":{"schema":"getpostman.com"},"item":[null,{"name":"y","request":{"url":"http://h/y"}}]}`,
		"postman nested empty": `{"info":{"schema":"getpostman.com"},"item":[{"name":"f","item":[]}]}`,
		"insomnia bare array":  `[{"_type":"request","_id":"r","name":"a","url":"http://h/a"}]`,
		"insomnia no name":     `{"_type":"export","resources":[{"_type":"request","_id":"r","url":"http://h"}]}`,
		"insomnia headers obj": `{"_type":"export","resources":[{"_type":"request","_id":"r","name":"h","headers":{"X-A":"1"}}]}`,
		"insomnia bad headers": `{"_type":"export","resources":[{"_type":"request","_id":"r","name":"h","headers":7}]}`,
	}
	for name, body := range cases {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "c.json")
		os.WriteFile(p, []byte(body), 0o644)
		rep, err := Import(p, filepath.Join(tmp, "hittable"))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		for rel, h := range loadTree(t, rep.Dir) {
			if h.Method == "" || h.Headers == nil || h.Params == nil {
				t.Errorf("%s: %s has empty method or nil maps: %+v", name, rel, h)
			}
		}
	}
}

// A folder whose parent chain loops (a corrupt or hand-edited export) must not
// recurse forever.
func TestInsomniaCyclicParentsTerminate(t *testing.T) {
	body := `{"_type":"export","__export_format":4,"resources":[
	  {"_type":"workspace","_id":"w","name":"Loop"},
	  {"_type":"request_group","_id":"a","parentId":"b","name":"A"},
	  {"_type":"request_group","_id":"b","parentId":"a","name":"B"},
	  {"_type":"request_group","_id":"s","parentId":"s","name":"Self"},
	  {"_type":"request","_id":"r1","parentId":"a","name":"in loop","url":"http://h/1"},
	  {"_type":"request","_id":"r2","parentId":"s","name":"in self","url":"http://h/2"}]}`
	tmp := t.TempDir()
	p := filepath.Join(tmp, "loop.json")
	os.WriteFile(p, []byte(body), 0o644)
	done := make(chan error, 1)
	var rep *Report
	go func() {
		var err error
		rep, err = Import(p, filepath.Join(tmp, "hittable"))
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("import did not terminate on a cyclic parent chain")
	}
	if got := loadTree(t, rep.Dir); len(got) != 2 {
		t.Errorf("want both requests written, got %v", keys(got))
	}
}

// Names from a collection are untrusted: nothing may land outside hittable/.
func TestImportNamesStayInsideHittableDir(t *testing.T) {
	body := `{"info":{"name":"../../escape","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":[
	  {"name":"../../../etc","item":[{"name":"../passwd","request":{"url":"http://h/1"}}]},
	  {"name":"a/b\\c","item":[{"name":"/abs/path","request":{"url":"http://h/2"}}]},
	  {"name":"..","request":{"url":"http://h/3"}},
	  {"name":"日本語のリクエスト","request":{"url":"http://h/4"}},
	  {"name":"","request":{"url":"http://h/5"}},
	  {"name":"CON:*?<>|\"","request":{"url":"http://h/6"}},
	  {"name":"` + strings.Repeat("long", 60) + `","request":{"url":"http://h/7"}}]}`
	tmp := t.TempDir()
	outer := filepath.Join(tmp, "outer")
	hd := filepath.Join(outer, "proj", "hittable")
	os.MkdirAll(hd, 0o755)
	p := filepath.Join(tmp, "evil.json")
	os.WriteFile(p, []byte(body), 0o644)
	rep, err := Import(p, hd)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Requests != 7 {
		t.Errorf("requests = %d", rep.Requests)
	}
	n := 0
	filepath.WalkDir(outer, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(hd, path)
		if strings.HasPrefix(rel, "..") {
			t.Errorf("file written outside hittable/: %s", path)
		}
		if filepath.Ext(path) == ".hit" {
			n++
			if len(filepath.Base(path)) > 90 {
				t.Errorf("file name not truncated: %d bytes", len(filepath.Base(path)))
			}
		}
		return nil
	})
	if n != 7 {
		t.Errorf("wrote %d .hit files, want 7 (empty / unicode names collapse but must not be lost)", n)
	}
}

func TestImportNameCollisions(t *testing.T) {
	body := `{"info":{"name":"Dup","schema":"getpostman.com"},"item":[
	  {"name":"Get","request":{"url":"http://h/1"}},
	  {"name":"Get","request":{"url":"http://h/2"}},
	  {"name":"Get","request":{"url":"http://h/3"}}]}`
	tmp := t.TempDir()
	p := filepath.Join(tmp, "dup.json")
	os.WriteFile(p, []byte(body), 0o644)
	hd := filepath.Join(tmp, "hittable")
	rep, err := Import(p, hd)
	if err != nil {
		t.Fatal(err)
	}
	got := loadTree(t, rep.Dir)
	for i, name := range []string{"Get.hit", "Get (2).hit", "Get (3).hit"} {
		if h := got[name]; h == nil || h.URL != "http://h/"+string(rune('1'+i)) {
			t.Errorf("%s = %+v", name, h)
		}
	}
	// Importing the same collection again never overwrites the first import.
	rep2, err := Import(p, hd)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(rep2.Dir) != "Dup (2)" || len(loadTree(t, rep.Dir)) != 3 {
		t.Errorf("second import dir = %s", rep2.Dir)
	}
}

func TestImportEnvMerge(t *testing.T) {
	body := `{"info":{"name":"E","schema":"getpostman.com"},"variable":[{"key":"BASE","value":"new"},{"key":"EXTRA","value":"x"}],"item":[]}`
	tmp := t.TempDir()
	p := filepath.Join(tmp, "e.json")
	os.WriteFile(p, []byte(body), 0o644)
	hd := filepath.Join(tmp, "hittable")
	os.MkdirAll(hd, 0o755)
	os.WriteFile(filepath.Join(hd, "env.json"), []byte(`{"BASE":"mine","KEEP":"k"}`), 0o644)
	rep, err := Import(p, hd)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Vars != 1 {
		t.Errorf("vars added = %d, want 1", rep.Vars)
	}
	if env := readEnvFile(t, hd); !reflect.DeepEqual(env, map[string]string{"BASE": "mine", "KEEP": "k", "EXTRA": "x"}) {
		t.Errorf("env = %v", env)
	}
}

// An env.json the importer can't read (mid-edit typo, a non-string value) must
// be left alone, not replaced by the imported variables.
func TestImportNeverClobbersUnreadableEnv(t *testing.T) {
	body := `{"info":{"name":"E","schema":"getpostman.com"},"variable":[{"key":"NEW","value":"v"}],"item":[{"name":"r","request":{"url":"{{NEW}}/x"}}]}`
	for name, env := range map[string]string{
		"syntax error":     `{"BASE":"mine",}`,
		"non-string value": `{"BASE":"mine","PORT":8080}`,
	} {
		tmp := t.TempDir()
		p := filepath.Join(tmp, "e.json")
		os.WriteFile(p, []byte(body), 0o644)
		hd := filepath.Join(tmp, "hittable")
		os.MkdirAll(hd, 0o755)
		envPath := filepath.Join(hd, "env.json")
		os.WriteFile(envPath, []byte(env), 0o644)
		_, err := Import(p, hd)
		if err == nil {
			t.Errorf("%s: import should report the unreadable env.json", name)
		}
		if b, _ := os.ReadFile(envPath); string(b) != env {
			t.Errorf("%s: env.json was rewritten to %s", name, b)
		}
	}
}

// Query strings in a raw URL are percent-decoded into params (FullURL encodes
// them again at send time, so a %20 must not become %2520).
func TestImportDecodesRawQuery(t *testing.T) {
	body := `{"_type":"export","resources":[{"_type":"request","_id":"r","name":"q","method":"GET",
	  "url":"http://h/search?q=a%20b&tag=x%26y&empty=&flag&bad=%zz"}]}`
	tmp := t.TempDir()
	p := filepath.Join(tmp, "q.json")
	os.WriteFile(p, []byte(body), 0o644)
	rep, err := Import(p, filepath.Join(tmp, "hittable"))
	if err != nil {
		t.Fatal(err)
	}
	h := loadTree(t, rep.Dir)["q.hit"]
	want := map[string]string{"q": "a b", "tag": "x&y", "empty": "", "flag": "", "bad": "%zz"}
	if h == nil || h.URL != "http://h/search" || !reflect.DeepEqual(h.Params, want) {
		t.Fatalf("got %+v", h)
	}
	if u := hitfile.Resolve(h, nil).FullURL(); strings.Contains(u, "%25") && !strings.Contains(u, "bad=%25zz") {
		t.Errorf("double-encoded on send: %s", u)
	}
}

// Postman URL objects without "raw" (some generators omit it) are rebuilt from
// host and path, as the web importer does.
func TestPostmanURLWithoutRaw(t *testing.T) {
	body := `{"info":{"name":"U","schema":"getpostman.com"},"item":[
	  {"name":"arr","request":{"url":{"protocol":"https","host":["api","example","test"],"path":["v1",{"value":"users"}],"query":[{"key":"a","value":"1"}]}}},
	  {"name":"str","request":{"url":{"host":"{{BASE}}","path":"items/2"}}}]}`
	tmp := t.TempDir()
	p := filepath.Join(tmp, "u.json")
	os.WriteFile(p, []byte(body), 0o644)
	rep, err := Import(p, filepath.Join(tmp, "hittable"))
	if err != nil {
		t.Fatal(err)
	}
	got := loadTree(t, rep.Dir)
	if h := got["arr.hit"]; h == nil || h.URL != "https://api.example.test/v1/users" || h.Params["a"] != "1" {
		t.Errorf("arr = %+v", h)
	}
	if h := got["str.hit"]; h == nil || h.URL != "<<BASE>>/items/2" {
		t.Errorf("str = %+v", h)
	}
}
