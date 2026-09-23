package hithome

import (
	"os"
	"path/filepath"
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

func TestDirFallsBackToHome(t *testing.T) {
	t.Setenv(EnvHome, "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no user home directory in this environment")
	}
	if got, want := Dir(), filepath.Join(home, ".hittable"); got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestDirIgnoresBlankEnv(t *testing.T) {
	t.Setenv(EnvHome, "   ")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no user home directory in this environment")
	}
	if got, want := Dir(), filepath.Join(home, ".hittable"); got != want {
		t.Errorf("Dir() with blank env = %q, want %q", got, want)
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
