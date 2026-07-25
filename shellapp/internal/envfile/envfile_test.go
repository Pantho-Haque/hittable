package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInterpolate(t *testing.T) {
	env := &EnvFile{
		Data: map[string]string{
			"BASE_URL": "https://api.example.com",
			"TOKEN":    "abc123",
		},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"<<BASE_URL>>/users", "https://api.example.com/users"},
		{"Bearer <<TOKEN>>", "Bearer abc123"},
		{"no interpolation", "no interpolation"},
		{"<<UNKNOWN>>", "<<UNKNOWN>>"},
		{"<<BASE_URL>>/users?token=<<TOKEN>>", "https://api.example.com/users?token=abc123"},
	}

	for _, tt := range tests {
		result := env.Interpolate(tt.input)
		if result != tt.expected {
			t.Errorf("Interpolate(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestLoadAndSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "envfile-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	envPath := filepath.Join(tmpDir, "env.json")
	original := `{
  "BASE_URL": "https://api.example.com",
  "TOKEN": "secret"
}`
	os.WriteFile(envPath, []byte(original), 0o644)

	env, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if env.Data["BASE_URL"] != "https://api.example.com" {
		t.Errorf("BASE_URL = %q, want %q", env.Data["BASE_URL"], "https://api.example.com")
	}
	if env.Data["TOKEN"] != "secret" {
		t.Errorf("TOKEN = %q, want %q", env.Data["TOKEN"], "secret")
	}
}

func TestLoadMissingFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "envfile-missing")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	env, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load with missing file should not error: %v", err)
	}

	if len(env.Data) != 0 {
		t.Errorf("expected empty data, got %v", env.Data)
	}
}
