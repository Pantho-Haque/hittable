package main

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hittable/shellapp/internal/hithome"
)

// appDataDirs lists every directory any version of hittable could have created
// for its own state — not just the one in use now. Uninstall has to clear all
// of them: a layout changed between releases would otherwise strand a 2GB
// model cache that nothing left on the machine knows about.
//
// Project files (hittable/ folders inside a user's repos) are never touched.
func appDataDirs() []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" {
			return
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		// Never return a root or a home directory, however the environment is
		// set: this path feeds os.RemoveAll.
		if home, err := os.UserHomeDir(); err == nil && (p == home || p == filepath.Dir(p)) {
			return
		}
		if seen[p] {
			return
		}
		seen[p] = true
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			out = append(out, p)
		}
	}

	add(hithome.Dir()) // the current layout, and $HITTABLE_HOME if set
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".hittable"))
		add(filepath.Join(home, ".config", "hittable"))
		add(filepath.Join(home, ".local", "share", "hittable"))
		add(filepath.Join(home, ".cache", "hittable"))
	}
	for _, env := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		if v := os.Getenv(env); v != "" {
			add(filepath.Join(v, "hittable"))
		}
	}
	return out
}

// removeAppData deletes those directories and reports the bytes reclaimed.
// Sizes are measured before deletion, since afterwards there is nothing to
// walk. A directory that fails to delete is not reported as freed.
func removeAppData() (freed int64, removed []string) {
	for _, d := range appDataDirs() {
		size := dirSize(d)
		if err := os.RemoveAll(d); err != nil {
			continue
		}
		freed += size
		removed = append(removed, d)
	}
	return freed, removed
}

func dirSize(root string) int64 {
	var n int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			n += info.Size()
		}
		return nil
	})
	return n
}
