package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureIdempotent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scaffold-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Ensure(tmpDir); err != nil {
		t.Fatalf("first Ensure failed: %v", err)
	}

	hitPath := filepath.Join(tmpDir, "hittable", "testcollection", "test.hit")
	envPath := filepath.Join(tmpDir, "hittable", "env.json")
	notesPath := filepath.Join(tmpDir, "hittable", "notes", "sample.md")

	for _, p := range []string{hitPath, envPath, notesPath} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist after first Ensure", p)
		}
	}

	hit1, _ := os.ReadFile(hitPath)
	env1, _ := os.ReadFile(envPath)
	notes1, _ := os.ReadFile(notesPath)

	if err := Ensure(tmpDir); err != nil {
		t.Fatalf("second Ensure failed: %v", err)
	}

	hit2, _ := os.ReadFile(hitPath)
	env2, _ := os.ReadFile(envPath)
	notes2, _ := os.ReadFile(notesPath)

	if string(hit1) != string(hit2) {
		t.Error("test.hit was modified on second Ensure")
	}
	if string(env1) != string(env2) {
		t.Error("env.json was modified on second Ensure")
	}
	if string(notes1) != string(notes2) {
		t.Error("sample.md was modified on second Ensure")
	}
}

func TestEnsureExistingDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scaffold-existing")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	hittableDir := filepath.Join(tmpDir, "hittable")
	os.MkdirAll(hittableDir, 0o755)

	existingFile := filepath.Join(hittableDir, "existing.txt")
	os.WriteFile(existingFile, []byte("original content"), 0o644)

	if err := Ensure(tmpDir); err != nil {
		t.Fatalf("Ensure with existing hittable/ failed: %v", err)
	}

	content, _ := os.ReadFile(existingFile)
	if string(content) != "original content" {
		t.Error("existing file was overwritten")
	}
}
