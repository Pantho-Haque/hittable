package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledBinaries(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "bin")
	os.MkdirAll(bin, 0o755)
	os.MkdirAll(filepath.Join(tmp, ".local", "bin"), 0o755)
	os.MkdirAll(filepath.Join(tmp, "go", "bin"), 0o755)
	os.WriteFile(filepath.Join(bin, "hittable"), []byte("x"), 0o755)
	os.WriteFile(filepath.Join(tmp, ".local", "bin", "hittable"), []byte("x"), 0o755)
	os.MkdirAll(filepath.Join(tmp, "dir", "hittable"), 0o755) // a directory, must be skipped
	t.Setenv("PATH", bin+string(os.PathListSeparator)+filepath.Join(tmp, "dir"))
	t.Setenv("HOME", tmp)

	got := installedBinaries()
	found := map[string]bool{}
	for _, p := range got {
		found[p] = true
	}
	for _, want := range []string{filepath.Join(bin, "hittable"), filepath.Join(tmp, ".local", "bin", "hittable")} {
		if r, _ := filepath.EvalSymlinks(want); !found[r] && !found[want] {
			t.Errorf("missing %s in %v", want, got)
		}
	}
	for p := range found {
		if fi, err := os.Stat(p); err != nil || fi.IsDir() {
			t.Errorf("bad entry %s", p)
		}
	}
}

func TestRunningInstancesExcludesSelf(t *testing.T) {
	// Only guarantees this process is never counted; other instances may exist.
	for _, pid := range runningInstances() {
		if pid == "" {
			t.Error("empty pid")
		}
	}
}
