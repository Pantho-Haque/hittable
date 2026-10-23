package hithome

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirHonoursEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv(EnvHome, home)

	if got := Dir(); got != home {
		t.Errorf("Dir() = %q, want %q", got, home)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Runtime", Runtime(), filepath.Join(home, "runtime")},
		{"RuntimeTag", RuntimeTag("b6981"), filepath.Join(home, "runtime", "b6981")},
		{"RuntimeCurrent", RuntimeCurrent(), filepath.Join(home, "runtime", "current")},
		{"Models", Models(), filepath.Join(home, "models")},
		{"Model", Model("m.gguf"), filepath.Join(home, "models", "m.gguf")},
		{"Run", Run(), filepath.Join(home, "run")},
		{"RunFile", RunFile(), filepath.Join(home, "run", "llama.json")},
		{"Tmp", Tmp(), filepath.Join(home, "tmp")},
		{"Logs", Logs(), filepath.Join(home, "logs")},
		{"ConfigPath", ConfigPath(), filepath.Join(home, "config.json")},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s() = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestUserDirIsUnderHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no user home directory in this environment")
	}
	if got, want := userDir(), filepath.Join(home, ".hittable"); got != want {
		t.Errorf("userDir() = %q, want %q", got, want)
	}
}

// Under go test, an unset or blank HITTABLE_HOME must never resolve to the
// developer's real ~/.hittable: that would read their config and start their
// installed model.
func TestDirNeverUsesRealHomeInTests(t *testing.T) {
	t.Setenv("HITTABLE_AI_E2E", "")
	for _, v := range []string{"", "   "} {
		t.Setenv(EnvHome, v)
		got := Dir()
		if got == userDir() {
			t.Fatalf("Dir() with HITTABLE_HOME=%q = real home %q", v, got)
		}
		if !strings.HasPrefix(got, os.TempDir()) {
			t.Errorf("Dir() = %q, want a path under %q", got, os.TempDir())
		}
	}
}

func TestEnsureDirsIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv(EnvHome, filepath.Join(home, "nested", "state"))

	for i := range 2 {
		if err := EnsureDirs(); err != nil {
			t.Fatalf("EnsureDirs() call %d: %v", i+1, err)
		}
	}

	for _, d := range []string{Dir(), Runtime(), Models(), Run(), Tmp(), Logs()} {
		fi, err := os.Stat(d)
		if err != nil {
			t.Fatalf("stat %s: %v", d, err)
		}
		if !fi.IsDir() {
			t.Errorf("%s is not a directory", d)
		}
	}
}
