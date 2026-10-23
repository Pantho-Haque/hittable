package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// run executes the CLI like a user would, capturing stdout. Flag variables
// are package globals, so they are reset before every run.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	t.Setenv("HITTABLE_HOME", t.TempDir())
	t.Setenv("HITTABLE_AI", "0")
	flagImport, flagExport, flagOut, flagIcons = "", "", "", "nerd"
	flagModelYes, flagModelDryRun = false, false
	// cobra keeps --help / --version set on the command between Executes.
	var reset func(c *cobra.Command)
	reset = func(c *cobra.Command) {
		for _, f := range []string{"help", "version"} {
			if fl := c.Flags().Lookup(f); fl != nil {
				fl.Value.Set("false")
				fl.Changed = false
			}
		}
		for _, sub := range c.Commands() {
			reset(sub)
		}
	}
	reset(rootCmd)

	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	rootCmd.SetOut(w)
	rootCmd.SetArgs(args)
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	err := rootCmd.Execute()
	w.Close()
	os.Stdout = old
	rootCmd.SetOut(nil)
	return <-done, err
}

func TestCLIInit(t *testing.T) {
	root := t.TempDir()
	out, err := run(t, "init", root)
	if err != nil || !strings.Contains(out, "created") {
		t.Fatalf("init: %q %v", out, err)
	}
	for _, f := range []string{"hittable/env.json", "hittable/testcollection/test.hit", "hittable/notes/sample.md"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	// A second init never overwrites the user's files.
	env := filepath.Join(root, "hittable", "env.json")
	os.WriteFile(env, []byte(`{"MINE":"1"}`), 0o644)
	out, err = run(t, "init", root)
	if err != nil || !strings.Contains(out, "already exists") {
		t.Errorf("re-init: %q %v", out, err)
	}
	if b, _ := os.ReadFile(env); string(b) != `{"MINE":"1"}` {
		t.Errorf("env.json overwritten: %s", b)
	}

	// init with no path uses the working directory.
	cwd := t.TempDir()
	t.Chdir(cwd)
	if _, err := run(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cwd, "hittable", "env.json")); err != nil {
		t.Error("init without a path did not use the working directory")
	}
}

func TestCLIInitBadArgs(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "f")
	os.WriteFile(file, nil, 0o644)
	for _, args := range [][]string{
		{"init", filepath.Join(tmp, "missing")},
		{"init", file},
		{"init", tmp, tmp},
	} {
		if _, err := run(t, args...); err == nil {
			t.Errorf("%v: want error", args)
		}
	}
	if _, err := os.Stat(filepath.Join(tmp, "missing")); err == nil {
		t.Error("init created a missing directory")
	}
}

func readJSON(t *testing.T, p string) map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s is not JSON: %v", p, err)
	}
	return m
}

func TestCLIExport(t *testing.T) {
	root := filepath.Join(t.TempDir(), "myapi")
	os.Mkdir(root, 0o755)
	if _, err := run(t, "init", root); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	name := filepath.Base(root)

	// -e alone defaults to Postman, written next to hittable/.
	out, err := run(t, "-e")
	if err != nil || !strings.Contains(out, "exported 1 requests") {
		t.Fatalf("-e: %q %v", out, err)
	}
	pm := readJSON(t, filepath.Join(root, name+".postman_collection.json"))
	if _, ok := pm["item"]; !ok {
		t.Errorf("postman export has no items: %v", pm)
	}

	// -e insomnia (bare format word), with and without a path argument.
	if out, err := run(t, "-e", "insomnia"); err != nil || !strings.Contains(out, ".insomnia.json") {
		t.Fatalf("-e insomnia: %q %v", out, err)
	}
	readJSON(t, filepath.Join(root, name+".insomnia.json"))
	os.Remove(filepath.Join(root, name+".insomnia.json"))
	t.Chdir(t.TempDir())
	if out, err := run(t, "-e", "insomnia", root); err != nil || !strings.Contains(out, "exported 1") {
		t.Fatalf("-e insomnia <path>: %q %v", out, err)
	}
	readJSON(t, filepath.Join(root, name+".insomnia.json"))
	if _, err := run(t, "-e", "postman", root); err != nil {
		t.Errorf("-e postman <path>: %v", err)
	}
	if _, err := run(t, "--export=INSOMNIA", root); err != nil {
		t.Errorf("--export=INSOMNIA: %v", err)
	}

	// -o picks the output file.
	custom := filepath.Join(t.TempDir(), "x.json")
	if _, err := run(t, "-e", "postman", "-o", custom, root); err != nil {
		t.Fatal(err)
	}
	readJSON(t, custom)

	// Bad export invocations.
	if _, err := run(t, "--export=yaml", root); err == nil || !strings.Contains(err.Error(), "unknown export format") {
		t.Errorf("unknown format: %v", err)
	}
	if _, err := run(t, "-e", "postman", "-o", filepath.Join(root, "no", "such", "dir.json"), root); err == nil {
		t.Error("unwritable -o: want error")
	}
	if _, err := run(t, "-e", "insomnia", root, root); err == nil {
		t.Error("two paths: want error")
	}
}

func TestCLIExportWithoutHittableDir(t *testing.T) {
	root := t.TempDir()
	_, err := run(t, "-e", root)
	if err == nil || !strings.Contains(err.Error(), "hittable init") {
		t.Errorf("want pointer to `hittable init`, got %v", err)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Errorf("export wrote files into an empty dir: %v", entries)
	}
}

func TestCLIImportErrorsDoNotOpenTUI(t *testing.T) {
	root := t.TempDir()
	bad := filepath.Join(root, "bad.json")
	os.WriteFile(bad, []byte("{not json"), 0o644)
	for _, f := range []string{bad, filepath.Join(root, "missing.json")} {
		if _, err := run(t, "-i", f, root); err == nil || !strings.HasPrefix(err.Error(), "import:") {
			t.Errorf("-i %s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "hittable")); err == nil {
		t.Error("a failed import created hittable/")
	}
}

func TestCLIRootBadArgs(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "f")
	os.WriteFile(file, nil, 0o644)
	for _, args := range [][]string{
		{filepath.Join(tmp, "missing")},
		{file},
		{tmp, tmp},
		{"--no-such-flag"},
		{"version", "extra"},
		{"uninstall", "extra"},
		{"update", "extra"},
		{"model", "status", "extra"},
	} {
		if _, err := run(t, args...); err == nil {
			t.Errorf("%v: want error", args)
		}
	}
}

func TestCLIVersionAndHelp(t *testing.T) {
	out, err := run(t, "version")
	want := "hittable " + version + " " + runtime.GOOS + "/" + runtime.GOARCH
	if err != nil || strings.TrimSpace(out) != want {
		t.Errorf("version = %q, want %q (%v)", out, want, err)
	}
	if out, err := run(t, "--version"); err != nil || !strings.Contains(out, version) {
		t.Errorf("--version = %q %v", out, err)
	}
	out, err = run(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"init", "-i collection.json", "-e postman", "-e insomnia", "model enable", "update", "uninstall", "--icons", "--out"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q", want)
		}
	}
	out, _ = run(t, "model", "--help")
	for _, want := range []string{"enable", "status", "disable", "delete"} {
		if !strings.Contains(out, want) {
			t.Errorf("model help missing %q", want)
		}
	}
}

// The model commands must work on a machine with nothing installed, and
// never download or start anything.
func TestCLIModelCommandsWithNothingInstalled(t *testing.T) {
	out, err := run(t, "model", "status")
	if err != nil || !strings.Contains(out, "not installed") || !strings.Contains(out, "not running") {
		t.Errorf("status: %q %v", out, err)
	}
	out, err = run(t, "model", "disable")
	if err != nil || !strings.Contains(out, "was not running") {
		t.Errorf("disable: %q %v", out, err)
	}
	out, err = run(t, "model", "delete", "--yes")
	if err != nil || !strings.Contains(out, "Nothing installed") {
		t.Errorf("delete: %q %v", out, err)
	}
}
