package screens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	zone "github.com/lrstanley/bubblezone"
)

func TestParamsLivePersist(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(80, 24)

	hitFile := filepath.Join(hittableDir, "test.hit")
	initial := `{"method":"GET","url":"http://test.com","headers":{},"params":{},"body":"","response":null}`
	os.WriteFile(hitFile, []byte(initial), 0o644)
	m.openFileRaw(hitFile)

	m.Params.SetContent(map[string]string{"foo": "bar"})
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow()

	time.Sleep(100 * time.Millisecond)

	data, err := os.ReadFile(hitFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var hit map[string]interface{}
	if err := json.Unmarshal(data, &hit); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	params, ok := hit["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected params to be map, got %T", hit["params"])
	}
	if params["foo"] != "bar" {
		t.Errorf("expected params.foo=bar, got %v", params["foo"])
	}
}

func TestHeadersLivePersist(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(80, 24)

	hitFile := filepath.Join(hittableDir, "test.hit")
	initial := `{"method":"GET","url":"http://test.com","headers":{},"params":{},"body":"","response":null}`
	os.WriteFile(hitFile, []byte(initial), 0o644)
	m.openFileRaw(hitFile)

	m.Headers.SetContent(map[string]string{"X-Custom": "test-value"})
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow()

	time.Sleep(100 * time.Millisecond)

	data, err := os.ReadFile(hitFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var hit map[string]interface{}
	if err := json.Unmarshal(data, &hit); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	headers, ok := hit["headers"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected headers to be map, got %T", hit["headers"])
	}
	if headers["X-Custom"] != "test-value" {
		t.Errorf("expected X-Custom=test-value, got %v", headers["X-Custom"])
	}
}

func TestBodyLivePersist(t *testing.T) {
	tmpDir := t.TempDir()
	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)
	os.WriteFile(filepath.Join(hittableDir, "env.json"), []byte(`{}`), 0o644)

	z := zone.New()
	m := NewMainScreen(tmpDir, z)
	m.SetSize(80, 24)

	hitFile := filepath.Join(hittableDir, "test.hit")
	initial := `{"method":"GET","url":"http://test.com","headers":{},"params":{},"body":"","response":null}`
	os.WriteFile(hitFile, []byte(initial), 0o644)
	m.openFileRaw(hitFile)

	m.Body.SetContent(`{"hello": "world"}`)
	m.saveAndEnqueue()
	m.WriteQueue.FlushNow()

	time.Sleep(100 * time.Millisecond)

	data, err := os.ReadFile(hitFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var hit map[string]interface{}
	if err := json.Unmarshal(data, &hit); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	body, ok := hit["body"].(string)
	if !ok {
		t.Fatalf("expected body to be string, got %T", hit["body"])
	}
	if body != `{"hello": "world"}` {
		t.Errorf("expected body to be test content, got %v", body)
	}
}
