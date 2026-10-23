package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The template is what `hittable init` hands a new user: it must match the
// spec byte-for-byte in shape and be openable by the app and the web app.
func TestTemplateContent(t *testing.T) {
	root := t.TempDir()
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "hittable", "testcollection", "test.hit"))
	var hit map[string]any
	if err := json.Unmarshal(b, &hit); err != nil {
		t.Fatalf("test.hit: %v", err)
	}
	if hit["method"] != "GET" || hit["url"] != "https://jsonplaceholder.typicode.com/posts/1" || hit["response"] != nil || hit["body"] != "" {
		t.Errorf("test.hit = %v", hit)
	}
	if _, ok := hit["params"].(map[string]any); !ok {
		t.Error("params must be an object, not null")
	}
	if !strings.HasSuffix(string(b), "}\n") {
		t.Error("test.hit should end with a newline")
	}
	e, _ := os.ReadFile(filepath.Join(root, "hittable", "env.json"))
	var env map[string]string
	if err := json.Unmarshal(e, &env); err != nil || env["BASE_URL"] == "" {
		t.Errorf("env.json = %s (%v)", e, err)
	}
	n, _ := os.ReadFile(filepath.Join(root, "hittable", "notes", "sample.md"))
	if string(n) != "# Notes\n\nWrite your notes here.\n" {
		t.Errorf("sample.md = %q", n)
	}
}

func TestEnsureFailsCleanly(t *testing.T) {
	// A file named hittable is treated as existing: never overwritten.
	root := t.TempDir()
	p := filepath.Join(root, "hittable")
	os.WriteFile(p, []byte("mine"), 0o644)
	if err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "mine" {
		t.Error("existing hittable file was replaced")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	ro := t.TempDir()
	os.Chmod(ro, 0o555)
	t.Cleanup(func() { os.Chmod(ro, 0o755) })
	if err := Ensure(ro); err == nil {
		t.Error("Ensure in a read-only directory should return an error")
	}
	if err := Ensure(filepath.Join(root, "missing", "deeper")); err != nil {
		t.Errorf("Ensure creates missing parents: %v", err)
	}
}
