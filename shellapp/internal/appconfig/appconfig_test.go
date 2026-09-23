package appconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hittable/shellapp/internal/hithome"
)

// setup points the whole app state tree at a temp dir and clears the AI
// environment, so a developer's own HITTABLE_AI never changes a result.
func setup(t *testing.T) (home, root string) {
	t.Helper()
	home = t.TempDir()
	root = t.TempDir()
	t.Setenv(hithome.EnvHome, home)
	t.Setenv(EnvAI, "")
	t.Setenv(EnvAIEndpoint, "")
	t.Setenv(EnvAIModel, "")
	return home, root
}

func writeJSON(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoadPrecedence walks the table one row per layer, each row adding the
// layer above it, so every row asserts that the higher layer won and that the
// fields the higher layer did not set survived from below.
func TestLoadPrecedence(t *testing.T) {
	tests := []struct {
		name         string
		userFile     string // ~/.hittable/config.json
		projectFile  string // <root>/hittable/config.json
		env          map[string]string
		wantEnabled  bool
		wantEndpoint string
		wantModel    string
		wantTimeout  int
	}{
		{
			name:         "defaults only",
			wantEnabled:  true,
			wantEndpoint: "",
			wantModel:    Defaults().AI.Model,
			wantTimeout:  60000,
		},
		{
			name:         "user file over defaults",
			userFile:     `{"ai":{"enabled":false,"model":"user-model","timeoutMs":1234}}`,
			wantEnabled:  false,
			wantEndpoint: "",
			wantModel:    "user-model",
			wantTimeout:  1234,
		},
		{
			name:         "user file merges field by field",
			userFile:     `{"ai":{"endpoint":"http://127.0.0.1:9999"}}`,
			wantEnabled:  true,
			wantEndpoint: "http://127.0.0.1:9999",
			wantModel:    Defaults().AI.Model,
			wantTimeout:  60000,
		},
		{
			name:         "project file over user file",
			userFile:     `{"ai":{"enabled":false,"model":"user-model","timeoutMs":1234}}`,
			projectFile:  `{"ai":{"enabled":true,"model":"project-model"}}`,
			wantEnabled:  true,
			wantEndpoint: "",
			wantModel:    "project-model",
			wantTimeout:  1234, // not set by the project file, kept from the user file
		},
		{
			name:         "env over project file",
			userFile:     `{"ai":{"enabled":true,"model":"user-model"}}`,
			projectFile:  `{"ai":{"enabled":true,"model":"project-model","endpoint":"http://project"}}`,
			env:          map[string]string{EnvAI: "0", EnvAIModel: "env-model", EnvAIEndpoint: "http://env"},
			wantEnabled:  false,
			wantEndpoint: "http://env",
			wantModel:    "env-model",
			wantTimeout:  60000,
		},
		{
			name:         "HITTABLE_AI=0 wins over every file saying true",
			userFile:     `{"ai":{"enabled":true}}`,
			projectFile:  `{"ai":{"enabled":true}}`,
			env:          map[string]string{EnvAI: "0"},
			wantEnabled:  false,
			wantEndpoint: "",
			wantModel:    Defaults().AI.Model,
			wantTimeout:  60000,
		},
		{
			name:         "HITTABLE_AI=1 wins over every file saying false",
			userFile:     `{"ai":{"enabled":false}}`,
			projectFile:  `{"ai":{"enabled":false}}`,
			env:          map[string]string{EnvAI: "1"},
			wantEnabled:  true,
			wantEndpoint: "",
			wantModel:    Defaults().AI.Model,
			wantTimeout:  60000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, root := setup(t)
			if tt.userFile != "" {
				writeJSON(t, filepath.Join(home, FileName), tt.userFile)
			}
			if tt.projectFile != "" {
				writeJSON(t, ProjectPath(root), tt.projectFile)
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			c, warns := Load(root)
			if len(warns) != 0 {
				t.Errorf("Load() warnings = %v, want none", warns)
			}
			if c.AI.Enabled != tt.wantEnabled {
				t.Errorf("Enabled = %v, want %v", c.AI.Enabled, tt.wantEnabled)
			}
			if c.AI.Endpoint != tt.wantEndpoint {
				t.Errorf("Endpoint = %q, want %q", c.AI.Endpoint, tt.wantEndpoint)
			}
			if c.AI.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", c.AI.Model, tt.wantModel)
			}
			if c.AI.TimeoutMs != tt.wantTimeout {
				t.Errorf("TimeoutMs = %d, want %d", c.AI.TimeoutMs, tt.wantTimeout)
			}
		})
	}
}

func TestLoadNeverFailsOnBadInput(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantSub string
	}{
		{"truncated object", `{"ai":{"enabled":`, "not valid JSON"},
		{"array at top level", `[1,2,3]`, "not valid JSON"},
		{"wrong field type", `{"ai":{"timeoutMs":"soon"}}`, "not valid JSON"},
		{"html error page", "<!doctype html><h1>nope</h1>", "not valid JSON"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, root := setup(t)
			writeJSON(t, filepath.Join(home, FileName), tt.body)

			c, warns := Load(root)
			if len(warns) != 1 {
				t.Fatalf("Load() warnings = %v, want exactly one", warns)
			}
			if !strings.Contains(warns[0], tt.wantSub) {
				t.Errorf("warning = %q, want it to mention %q", warns[0], tt.wantSub)
			}
			if c != Defaults() {
				t.Errorf("Load() = %+v, want the defaults", c)
			}
		})
	}
}

func TestLoadIgnoresEmptyAndMissingFiles(t *testing.T) {
	home, root := setup(t)
	writeJSON(t, filepath.Join(home, FileName), "  \n")

	c, warns := Load(root)
	if len(warns) != 0 {
		t.Errorf("warnings = %v, want none", warns)
	}
	if c != Defaults() {
		t.Errorf("Load() = %+v, want the defaults", c)
	}
}

func TestLoadWarnsOnBadValues(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		check   func(Config) bool
		wantSub string
	}{
		{"zero timeout", `{"ai":{"timeoutMs":0}}`, func(c Config) bool { return c.AI.TimeoutMs == 60000 }, "ai.timeoutMs"},
		{"negative completion timeout", `{"ai":{"completionTimeoutMs":-5}}`, func(c Config) bool { return c.AI.CompletionTimeoutMs == 800 }, "ai.completionTimeoutMs"},
		{"zero prompt budget", `{"ai":{"maxPromptBytes":0}}`, func(c Config) bool { return c.AI.MaxPromptBytes == 12000 }, "ai.maxPromptBytes"},
		{"zero idle minutes", `{"ai":{"idleMinutes":0}}`, func(c Config) bool { return c.AI.IdleMinutes == 10 }, "ai.idleMinutes"},
		{"temperature out of range", `{"ai":{"temperature":7}}`, func(c Config) bool { return c.AI.Temperature == 0.2 }, "ai.temperature"},
		{"bad HITTABLE_AI", `{}`, func(c Config) bool { return c.AI.Enabled }, "HITTABLE_AI"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, root := setup(t)
			writeJSON(t, filepath.Join(home, FileName), tt.body)
			if tt.wantSub == "HITTABLE_AI" {
				t.Setenv(EnvAI, "maybe")
			}

			c, warns := Load(root)
			if len(warns) != 1 {
				t.Fatalf("warnings = %v, want exactly one", warns)
			}
			if !strings.Contains(warns[0], tt.wantSub) {
				t.Errorf("warning = %q, want it to mention %q", warns[0], tt.wantSub)
			}
			if !tt.check(c) {
				t.Errorf("value not repaired: %+v", c.AI)
			}
		})
	}
}

// Temperature 0 is greedy decoding, not an unset field, so it must survive.
func TestZeroTemperatureIsKept(t *testing.T) {
	home, root := setup(t)
	writeJSON(t, filepath.Join(home, FileName), `{"ai":{"temperature":0}}`)

	c, warns := Load(root)
	if len(warns) != 0 {
		t.Errorf("warnings = %v, want none", warns)
	}
	if c.AI.Temperature != 0 {
		t.Errorf("Temperature = %v, want 0", c.AI.Temperature)
	}
}

func TestSaveRoundTrips(t *testing.T) {
	_, root := setup(t)

	c := Defaults()
	c.AI.Enabled = false
	c.AI.Model = "saved-model"
	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, warns := Load(root)
	if len(warns) != 0 {
		t.Errorf("warnings = %v, want none", warns)
	}
	if got != c {
		t.Errorf("Load after Save = %+v, want %+v", got, c)
	}
}

// Save must leave nothing behind but the config file itself — a stray temp file
// in ~/.hittable would be reported by `model status` as unexplained bytes.
func TestSaveLeavesNoTempFile(t *testing.T) {
	home, _ := setup(t)

	if err := Save(Defaults()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".config-") {
			t.Errorf("left a temp file behind: %s", e.Name())
		}
	}
	if _, err := os.Stat(hithome.ConfigPath()); err != nil {
		t.Errorf("config file not written: %v", err)
	}
}

func TestProjectPath(t *testing.T) {
	tests := []struct {
		root string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"/tmp/proj", filepath.Join("/tmp/proj", "hittable", "config.json")},
	}
	for _, tt := range tests {
		if got := ProjectPath(tt.root); got != tt.want {
			t.Errorf("ProjectPath(%q) = %q, want %q", tt.root, got, tt.want)
		}
	}
}
