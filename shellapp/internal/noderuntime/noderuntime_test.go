//go:build !windows

package noderuntime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeBin writes an executable that prints out.
func fakeBin(t *testing.T, dir, name, out string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho "+out+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// isolate gives the test an empty HOME and PATH and no system locations.
func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HITTABLE_HOME", filepath.Join(home, ".hittable"))
	t.Setenv("PATH", filepath.Join(home, "empty"))
	t.Setenv(EnvOverride, "")
	searchSystem = false
	t.Cleanup(func() { searchSystem = true; Forget() })
	Forget()
	return home
}

func TestFindPrefersPATH(t *testing.T) {
	home := isolate(t)
	if _, err := Find(); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("empty machine: %v", err)
	}
	// nvm with two versions: the newest is taken.
	fakeBin(t, filepath.Join(home, ".nvm", "versions", "node", "v20.9.0", "bin"), "node", "v20.9.0")
	fakeBin(t, filepath.Join(home, ".nvm", "versions", "node", "v22.11.0", "bin"), "node", "v22.11.0")
	Forget()
	f, err := Find()
	if err != nil || f.Version != "v22.11.0" || f.Source != "nvm" {
		t.Fatalf("nvm: %+v %v", f, err)
	}
	// node on PATH wins over a version manager's other installs.
	bin := filepath.Join(home, "bin")
	fakeBin(t, bin, "node", "v24.1.0")
	t.Setenv("PATH", bin)
	Forget()
	if f, err := Find(); err != nil || f.Source != "PATH" || f.Version != "v24.1.0" {
		t.Fatalf("PATH: %+v %v", f, err)
	}
}

func TestFindRejectsOldNode(t *testing.T) {
	home := isolate(t)
	bin := filepath.Join(home, "bin")
	fakeBin(t, bin, "node", "v18.19.0")
	t.Setenv("PATH", bin)
	Forget()
	_, err := Find()
	if !errors.Is(err, ErrTooOld) || !strings.Contains(err.Error(), "v18.19.0") {
		t.Fatalf("old node: %v", err)
	}
	// A newer one elsewhere is still found.
	fakeBin(t, filepath.Join(home, ".volta", "bin"), "node", "v22.0.0")
	Forget()
	if f, err := Find(); err != nil || f.Source != "Volta" {
		t.Fatalf("volta: %+v %v", f, err)
	}
}

func TestOverride(t *testing.T) {
	home := isolate(t)
	t.Setenv(EnvOverride, fakeBin(t, filepath.Join(home, "o"), "node", "v18.0.0"))
	Forget()
	if _, err := Find(); err == nil || !strings.Contains(err.Error(), "need 20 or newer") {
		t.Errorf("an old override was accepted: %v", err)
	}
	ok := fakeBin(t, filepath.Join(home, "o2"), "node", "v22.1.0")
	t.Setenv(EnvOverride, ok)
	Forget()
	if f, err := Find(); err != nil || f.Path != ok || f.Source != EnvOverride {
		t.Errorf("Find() = %+v, %v", f, err)
	}
}

func TestPlanInstall(t *testing.T) {
	home := isolate(t)
	plan, err := PlanInstall()
	if err != nil {
		t.Skip("no official build for this platform: ", err)
	}
	if plan.Method != "download" || !strings.Contains(plan.Summary, "~/.local") {
		t.Errorf("without brew: %+v", plan)
	}
	// With Homebrew present, it is used; an installed-but-old node upgrades.
	bin := filepath.Join(home, "bin")
	fakeBin(t, bin, "brew", "node 18.0.0")
	t.Setenv("PATH", bin)
	if plan, _ := PlanInstall(); plan.Method != "brew" || !strings.Contains(plan.Command, "upgrade node") {
		t.Errorf("with brew: %+v", plan)
	}
}

func TestRemoveOnlyTouchesItsOwnLinks(t *testing.T) {
	home := isolate(t)
	ours := filepath.Join(home, ".local", "lib", "nodejs", "node-"+Version+"-x", "bin")
	fakeBin(t, ours, "node", Version)
	lb := filepath.Join(home, ".local", "bin")
	os.MkdirAll(lb, 0o755)
	os.Symlink(filepath.Join(ours, "node"), filepath.Join(lb, "node"))
	mine := fakeBin(t, lb, "npm", "user's own npm")
	removed, err := Remove()
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 {
		t.Errorf("removed %v", removed)
	}
	if _, err := os.Stat(mine); err != nil {
		t.Error("Remove deleted a file it did not create")
	}
}

func TestEveryReleasePlatformHasAnAsset(t *testing.T) {
	for _, p := range []string{"darwin/arm64", "darwin/amd64", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"} {
		a, ok := assets[p]
		if !ok || len(a.sum) != 64 || a.bytes < 10<<20 || !strings.Contains(a.file, Version) {
			t.Errorf("%s: %+v", p, a)
		}
	}
}
