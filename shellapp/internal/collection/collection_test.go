package collection

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hittable/shellapp/internal/hitfile"
)

const postmanSample = `{
  "info": {"name": "Pets API", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
  "auth": {"type": "bearer", "bearer": [{"key": "token", "value": "{{TOKEN}}"}]},
  "variable": [{"key": "BASE", "value": "https://api.pets.test"}, {"key": "TOKEN", "value": "abc"}],
  "item": [
    {"name": "Pets", "item": [
      {"name": "List pets", "event": [{"listen": "test", "script": {}}],
       "request": {"method": "GET", "url": {"raw": "{{BASE}}/pets?limit=10", "query": [{"key": "limit", "value": "10"}, {"key": "off", "value": "1", "disabled": true}]},
       "header": [{"key": "Accept", "value": "application/json"}]}},
      {"name": "Create pet", "request": {"method": "POST", "url": "{{BASE}}/pets",
       "header": [{"key": "Content-Type", "value": "application/json"}],
       "body": {"mode": "raw", "raw": "{\"name\": \"{{PET}}\"}"}}}
    ]},
    {"name": "Login", "request": {"method": "POST", "url": "{{BASE}}/login", "auth": {"type": "noauth"},
     "body": {"mode": "urlencoded", "urlencoded": [{"key": "user", "value": "u"}, {"key": "pass", "value": "p"}]}}}
  ]
}`

const insomniaSample = `{
  "_type": "export", "__export_format": 4,
  "resources": [
    {"_type": "workspace", "_id": "wrk_1", "name": "Shop"},
    {"_type": "environment", "_id": "env_1", "parentId": "wrk_1", "data": {"HOST": "https://shop.test", "KEY": "k1"}},
    {"_type": "request_group", "_id": "fld_1", "parentId": "wrk_1", "name": "Orders"},
    {"_type": "request", "_id": "req_1", "parentId": "fld_1", "name": "Get order", "method": "GET",
     "url": "{{ _.HOST }}/orders/1", "headers": [{"name": "X-Key", "value": "{{ _.KEY }}"}],
     "parameters": [{"name": "expand", "value": "items"}],
     "authentication": {"type": "bearer", "token": "{{ _.KEY }}"}},
    {"_type": "request", "_id": "req_2", "parentId": "wrk_1", "name": "Ping", "method": "GET", "url": "{{ _.HOST }}/ping",
     "body": {"mimeType": "application/json", "text": "{\"a\":1}"}},
    {"_type": "cookie_jar", "_id": "jar_1"}
  ]
}`

func readHit(t *testing.T, p string) *hitfile.HitFile {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	h, err := hitfile.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestImportPostman(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "pets.postman_collection.json")
	os.WriteFile(src, []byte(postmanSample), 0o644)
	hd := filepath.Join(root, "hittable")
	rep, err := Import(src, hd)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Format != "Postman v2.1" || rep.Requests != 3 || rep.Folders != 1 || rep.Vars != 2 {
		t.Fatalf("report: %+v", rep)
	}
	if !strings.Contains(strings.Join(rep.Dropped, ","), "test script") {
		t.Errorf("dropped scripts not reported: %v", rep.Dropped)
	}
	list := readHit(t, filepath.Join(hd, "Pets API", "Pets", "List pets.hit"))
	if list.URL != "<<BASE>>/pets" || list.Params["limit"] != "10" || list.Params["off"] != "" {
		t.Errorf("list: %+v", list)
	}
	if list.Headers["Authorization"] != "Bearer <<TOKEN>>" || list.Headers["Accept"] != "application/json" {
		t.Errorf("inherited auth / headers: %+v", list.Headers)
	}
	create := readHit(t, filepath.Join(hd, "Pets API", "Pets", "Create pet.hit"))
	if create.Method != "POST" || !strings.Contains(create.Body, "<<PET>>") {
		t.Errorf("create: %+v", create)
	}
	login := readHit(t, filepath.Join(hd, "Pets API", "Login.hit"))
	if login.Headers["Content-Type"] != "application/x-www-form-urlencoded" || !strings.Contains(login.Body, `"user": "u"`) {
		t.Errorf("login: %+v", login)
	}
	if _, ok := login.Headers["Authorization"]; ok {
		t.Errorf("noauth request must not inherit auth: %+v", login.Headers)
	}
	env := map[string]string{}
	b, _ := os.ReadFile(filepath.Join(hd, "env.json"))
	json.Unmarshal(b, &env)
	if env["BASE"] != "https://api.pets.test" || env["TOKEN"] != "abc" {
		t.Errorf("env: %v", env)
	}

	// Round trip: export and re-import into a fresh dir yields the same requests.
	out, n, err := ExportPostman(hd, "Pets API")
	if err != nil || n != 3 {
		t.Fatalf("export: %v %d", err, n)
	}
	if Detect(out) != "postman" || !strings.Contains(string(out), "{{BASE}}/pets?limit=10") {
		t.Errorf("exported: %s", out)
	}
	exp := filepath.Join(root, "roundtrip.json")
	os.WriteFile(exp, out, 0o644)
	hd2 := filepath.Join(root, "hittable2")
	rep2, err := Import(exp, hd2)
	if err != nil || rep2.Requests != 3 {
		t.Fatalf("re-import: %v %+v", err, rep2)
	}
	again := readHit(t, filepath.Join(hd2, "Pets API", "Pets API", "Pets", "List pets.hit"))
	if again.URL != list.URL || again.Params["limit"] != "10" || again.Headers["Authorization"] != list.Headers["Authorization"] {
		t.Errorf("round trip changed the request: %+v", again)
	}
}

func TestImportInsomnia(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "shop.insomnia.json")
	os.WriteFile(src, []byte(insomniaSample), 0o644)
	hd := filepath.Join(root, "hittable")
	rep, err := Import(src, hd)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Format != "Insomnia" || rep.Requests != 2 || rep.Folders != 1 || rep.Vars != 2 || !strings.Contains(strings.Join(rep.Dropped, ","), "cookie_jar") {
		t.Fatalf("report: %+v", rep)
	}
	order := readHit(t, filepath.Join(hd, "Shop", "Orders", "Get order.hit"))
	if order.URL != "<<HOST>>/orders/1" || order.Params["expand"] != "items" || order.Headers["X-Key"] != "<<KEY>>" || order.Headers["Authorization"] != "Bearer <<KEY>>" {
		t.Errorf("order: %+v", order)
	}
	ping := readHit(t, filepath.Join(hd, "Shop", "Ping.hit"))
	if ping.Body != `{"a":1}` || ping.Headers["Content-Type"] != "application/json" {
		t.Errorf("ping: %+v", ping)
	}
	out, n, err := ExportInsomnia(hd, "Shop")
	if err != nil || n != 2 || Detect(out) != "insomnia" || !strings.Contains(string(out), "{{ _.HOST }}/orders/1") {
		t.Fatalf("export insomnia: %v %d\n%s", err, n, out)
	}
}

func TestDetectUnknown(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "x.json")
	os.WriteFile(p, []byte(`{"hello": 1}`), 0o644)
	if _, err := Import(p, root); err == nil {
		t.Error("expected an error for an unknown format")
	}
}

// Formdata imports as a live multipart body: fields in order, files as @src,
// a text value starting with @ escaped, and the multipart header set. A
// Postman file body and an Insomnia binary body become `@path`.
func TestImportFileBodies(t *testing.T) {
	root := t.TempDir()
	pm := `{"info": {"name": "Up", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
	  "item": [
	    {"name": "Form", "request": {"method": "POST", "url": "x.test", "body": {"mode": "formdata", "formdata": [
	      {"key": "zname", "value": "{{WHO}}"},
	      {"key": "avatar", "type": "file", "src": "/tmp/a.png", "contentType": "image/png"},
	      {"key": "handle", "value": "@me"},
	      {"key": "many", "type": "file", "src": ["files/1.txt", "files/2.txt"]},
	      {"key": "off", "value": "x", "disabled": true}]}}},
	    {"name": "Bin", "request": {"method": "PUT", "url": "x.test", "body": {"mode": "file", "file": {"src": "files/r.pdf"}}}}
	  ]}`
	src := filepath.Join(root, "up.postman_collection.json")
	os.WriteFile(src, []byte(pm), 0o644)
	hd := filepath.Join(root, "hittable")
	if _, err := Import(src, hd); err != nil {
		t.Fatal(err)
	}
	form := readHit(t, filepath.Join(hd, "Up", "Form.hit"))
	want := "{\n  \"zname\": \"<<WHO>>\",\n  \"avatar\": \"@/tmp/a.png;type=image/png\",\n  \"handle\": \"@@me\",\n  \"many\": \"@files/1.txt\"\n}"
	if form.Body != want || form.Headers["Content-Type"] != "multipart/form-data" {
		t.Errorf("form: %q %v", form.Body, form.Headers)
	}
	refs, err := hitfile.ParseFileRefs(form.Body, form.Headers["Content-Type"])
	if err != nil || len(refs.Files()) != 2 || refs.Parts[2].Value != "@me" {
		t.Errorf("imported form does not parse as intended: %v %+v", err, refs)
	}
	if bin := readHit(t, filepath.Join(hd, "Up", "Bin.hit")); bin.Body != "@files/r.pdf" {
		t.Errorf("file body: %q", bin.Body)
	}

	in := `{"_type": "export", "__export_format": 4, "resources": [
	  {"_type": "workspace", "_id": "wrk_1", "name": "Ins"},
	  {"_type": "request", "_id": "r1", "parentId": "wrk_1", "name": "Multi", "method": "POST", "url": "x.test",
	   "body": {"mimeType": "multipart/form-data", "params": [{"name": "b", "value": "@x"}, {"name": "a", "type": "file", "fileName": "/tmp/a.png"}]}},
	  {"_type": "request", "_id": "r2", "parentId": "wrk_1", "name": "Bin", "method": "PUT", "url": "x.test",
	   "body": {"mimeType": "application/pdf", "fileName": "/tmp/r.pdf"}}]}`
	src = filepath.Join(root, "ins.json")
	os.WriteFile(src, []byte(in), 0o644)
	hd = filepath.Join(root, "hittable2")
	if _, err := Import(src, hd); err != nil {
		t.Fatal(err)
	}
	multi := readHit(t, filepath.Join(hd, "Ins", "Multi.hit"))
	if multi.Body != "{\n  \"b\": \"@@x\",\n  \"a\": \"@/tmp/a.png\"\n}" || multi.Headers["Content-Type"] != "multipart/form-data" {
		t.Errorf("insomnia multipart: %q %v", multi.Body, multi.Headers)
	}
	bin := readHit(t, filepath.Join(hd, "Ins", "Bin.hit"))
	if bin.Body != "@/tmp/r.pdf" || bin.Headers["Content-Type"] != "application/pdf" {
		t.Errorf("insomnia binary: %q %v", bin.Body, bin.Headers)
	}
}
