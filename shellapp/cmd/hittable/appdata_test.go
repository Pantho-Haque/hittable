package main

import (
	"os"
	"path/filepath"
	"testing"
)

// sandboxHome redirects everything appDataDirs consults at a temp dir. Without
// this the tests below would enumerate — and then delete — the real ~/.hittable
// on the machine running them.
func sandboxHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // windows
	t.Setenv("HITTABLE_HOME", "")
	for _, e := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(e, "")
	}
	return home
}

func mkdirWithFile(t *testing.T, dir string, size int) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "blob"), make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Uninstall must clear every layout any version could have written, not just
// the current one, or a rename between releases strands the model cache.
func TestAppDataDirsCoversEveryLayout(t *testing.T) {
	home := sandboxHome(t)
	want := []string{
		filepath.Join(home, ".hittable"),
		filepath.Join(home, ".config", "hittable"),
		filepath.Join(home, ".local", "share", "hittable"),
		filepath.Join(home, ".cache", "hittable"),
	}
	for _, d := range want {
		mkdirWithFile(t, d, 16)
	}
	got := appDataDirs()
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
			}
		}
		if !found {
			t.Errorf("appDataDirs() missing %s; got %v", w, got)
		}
	}
}

func TestAppDataDirsHonoursEnvOverrides(t *testing.T) {
	sandboxHome(t)
	hh := filepath.Join(t.TempDir(), "custom-home")
	xdg := t.TempDir()
	mkdirWithFile(t, hh, 8)
	mkdirWithFile(t, filepath.Join(xdg, "hittable"), 8)
	t.Setenv("HITTABLE_HOME", hh)
	t.Setenv("XDG_DATA_HOME", xdg)

	got := appDataDirs()
	for _, w := range []string{hh, filepath.Join(xdg, "hittable")} {
		found := false
		for _, g := range got {
			if g == w {
				found = true
			}
		}
		if !found {
			t.Errorf("appDataDirs() missing %s; got %v", w, got)
		}
	}
}

// The list feeds os.RemoveAll, so it must never contain a home directory or a
// filesystem root however the environment is set.
func TestAppDataDirsNeverReturnsHomeOrRoot(t *testing.T) {
	home := sandboxHome(t)
	for _, bad := range []string{home, "/", string(filepath.Separator)} {
		t.Setenv("HITTABLE_HOME", bad)
		for _, g := range appDataDirs() {
			if g == filepath.Clean(bad) {
				t.Fatalf("appDataDirs() returned %q with HITTABLE_HOME=%q — RemoveAll would wipe it", g, bad)
			}
		}
	}
}

func TestRemoveAppDataDeletesAndReportsSize(t *testing.T) {
	home := sandboxHome(t)
	a := filepath.Join(home, ".hittable")
	b := filepath.Join(home, ".cache", "hittable")
	mkdirWithFile(t, filepath.Join(a, "models"), 2048)
	mkdirWithFile(t, b, 1024)

	// A project folder outside the app's own dirs must survive.
	project := filepath.Join(home, "work", "hittable")
	mkdirWithFile(t, project, 32)

	freed, removed := removeAppData()
	if len(removed) < 2 {
		t.Errorf("removed = %v, want both app dirs", removed)
	}
	if freed < 3072 {
		t.Errorf("freed = %d, want at least 3072 bytes", freed)
	}
	for _, d := range []string{a, b} {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Errorf("%s still exists", d)
		}
	}
	if _, err := os.Stat(project); err != nil {
		t.Errorf("a project hittable/ folder was deleted: %v", err)
	}
}

func TestRemoveAppDataOnAFreshMachine(t *testing.T) {
	sandboxHome(t)
	freed, removed := removeAppData()
	if freed != 0 || len(removed) != 0 {
		t.Errorf("removeAppData() on a fresh machine = %d, %v, want 0, none", freed, removed)
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1000, "1 KB"},
		{1_500_000, "2 MB"},
		{2_104_932_800, "2.10 GB"}, // the real model size
		{-1, "—"},
	}
	for _, tt := range tests {
		if got := humanBytes(tt.in); got != tt.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
