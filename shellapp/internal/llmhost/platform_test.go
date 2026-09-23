package llmhost

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hittable/shellapp/internal/hithome"
)

func TestFreeDiskIsMeasuredNotAssumed(t *testing.T) {
	dir := home(t)

	got := freeDisk(dir)
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if got <= 0 {
			t.Fatalf("freeDisk(%q) = %d, want a real measurement", dir, got)
		}
	}

	// The install target usually does not exist yet when the preview is drawn,
	// so the measurement has to walk up to something that does. Compared
	// loosely rather than for equality: free space genuinely moves between two
	// calls on a machine that is doing anything at all.
	deep := freeDisk(filepath.Join(dir, "not", "created", "yet"))
	if (deep > 0) != (got > 0) {
		t.Errorf("freeDisk of a missing subdirectory = %d, but of its parent = %d", deep, got)
	}
	if got > 0 && (deep < got/2 || deep > got*2) {
		t.Errorf("freeDisk of a missing subdirectory = %d, not plausibly the same filesystem as %d", deep, got)
	}
}

func TestNearestExisting(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		in   string
		want string
	}{
		{dir, dir},
		{filepath.Join(dir, "a", "b", "c"), dir},
	}
	for _, tt := range tests {
		if got := nearestExisting(tt.in); got != tt.want {
			t.Errorf("nearestExisting(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTotalRAM(t *testing.T) {
	got := totalRAM()
	switch runtime.GOOS {
	case "darwin", "linux":
		if got <= 0 {
			t.Fatalf("totalRAM() = %d on %s, want a real measurement", got, runtime.GOOS)
		}
		// Sanity: somewhere between a Raspberry Pi and a very large server.
		if got < 256<<20 || got > 8<<40 {
			t.Errorf("totalRAM() = %d, which is not a plausible amount of memory", got)
		}
	default:
		if got < 0 {
			t.Errorf("totalRAM() = %d; unknown must be 0, never negative", got)
		}
	}
}

func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Error("processAlive says this process is not running")
	}
	for _, pid := range []int{0, -1, -12345} {
		if processAlive(pid) {
			t.Errorf("processAlive(%d) = true", pid)
		}
	}
	// A pid far above any plausible live process.
	if processAlive(0x7FFFFFF) && runtime.GOOS != "windows" {
		t.Error("processAlive claims an unused pid is running")
	}
}

func TestIsSharedLib(t *testing.T) {
	tests := map[string]bool{
		"libllama.dylib":  true,
		"libggml.so":      true,
		"libggml.so.1":    true,
		"libggml.so.1.2":  true,
		"ggml-base.dll":   true,
		"llama-server":    false,
		"README.md":       false,
		"llama-cli.exe":   false,
		"model.gguf":      false,
		"libllama.dylib1": false,
	}
	for name, want := range tests {
		if got := isSharedLib(name); got != want {
			t.Errorf("isSharedLib(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestPostProcessIsBestEffortAndSetsTheExecutableBit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no executable bit on windows")
	}
	dir := t.TempDir()

	bin := filepath.Join(dir, "llama-server")
	lib := filepath.Join(dir, "libllama.dylib")
	doc := filepath.Join(dir, "README.md")
	for _, p := range []string{bin, lib, doc} {
		if err := os.WriteFile(p, []byte("not a real Mach-O"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// None of these files is a real binary, so codesign will refuse them. That
	// must not matter: every step here is best-effort, and the smoke test is
	// what reports a runtime that genuinely cannot start.
	postProcess(dir, bin)

	for _, p := range []string{bin, lib} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o755 {
			t.Errorf("%s mode = %v, want 0755", filepath.Base(p), fi.Mode().Perm())
		}
	}
	if fi, _ := os.Stat(doc); fi.Mode().Perm() == 0o755 {
		t.Error("documentation was made executable")
	}
}

func TestLibraryEnvPointsAtTheShippedLibraries(t *testing.T) {
	dir := filepath.FromSlash("/opt/hittable/runtime/b11120")

	var key string
	switch runtime.GOOS {
	case "darwin":
		key = "DYLD_LIBRARY_PATH"
	case "linux":
		key = "LD_LIBRARY_PATH"
	default:
		if got := libraryEnv([]string{"A=1"}, dir); len(got) != 1 {
			t.Errorf("libraryEnv added %v on %s", got[1:], runtime.GOOS)
		}
		return
	}

	t.Setenv(key, filepath.FromSlash("/existing/path"))
	got := libraryEnv([]string{"A=1"}, dir)
	if len(got) != 2 || got[0] != "A=1" {
		t.Fatalf("libraryEnv = %v, want the base environment plus one entry", got)
	}
	value, ok := strings.CutPrefix(got[1], key+"=")
	if !ok {
		t.Fatalf("libraryEnv set %q, want %s", got[1], key)
	}
	// Prepended, not appended: the shipped libllama must win over any other
	// copy already on the path.
	want := dir + string(os.PathListSeparator) + filepath.FromSlash("/existing/path")
	if value != want {
		t.Errorf("%s = %q, want %q", key, value, want)
	}
}

func TestDirSize(t *testing.T) {
	dir := t.TempDir()
	if _, err := dirSize(filepath.Join(dir, "missing")); err == nil {
		t.Error("dirSize of a missing path returned no error")
	}

	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "one"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "two"), make([]byte, 250), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := dirSize(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != 350 {
		t.Errorf("dirSize = %d, want 350", got)
	}
}

func TestOpenLogRingTrims(t *testing.T) {
	home(t)

	if err := os.WriteFile(logPath(), make([]byte, logLimit+4096), 0o644); err != nil {
		t.Fatal(err)
	}
	f := openLog()
	if f == nil {
		t.Fatal("openLog returned nil")
	}
	defer f.Close()

	fi, err := os.Stat(logPath())
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() > logLimit {
		t.Errorf("log is %d bytes after opening, want it trimmed under %d", fi.Size(), logLimit)
	}
	if fi.Size() == 0 {
		t.Error("the whole log was discarded; trimming keeps the recent half")
	}
	if !strings.HasPrefix(logPath(), hithome.Dir()) {
		t.Errorf("log path %q escaped HITTABLE_HOME", logPath())
	}
}
