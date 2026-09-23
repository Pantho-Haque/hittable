package llmhost

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hittable/shellapp/internal/hithome"
)

func TestPreviewUnsupportedPlatformIsNotAnError(t *testing.T) {
	home(t)

	tests := []struct {
		goos, goarch string
		supported    bool
	}{
		{"darwin", "arm64", true},
		{"darwin", "amd64", true},
		{"linux", "amd64", true},
		{"linux", "arm64", true},
		{"windows", "amd64", true},
		{"windows", "arm64", true},
		{"freebsd", "amd64", false},
		{"linux", "riscv64", false},
		{"plan9", "386", false},
	}

	for _, tt := range tests {
		p, err := preview(tt.goos, tt.goarch)
		if err != nil {
			t.Fatalf("preview(%s/%s) returned an error: %v", tt.goos, tt.goarch, err)
		}
		if p.Supported != tt.supported {
			t.Errorf("preview(%s/%s).Supported = %v, want %v", tt.goos, tt.goarch, p.Supported, tt.supported)
			continue
		}
		if tt.supported {
			if p.RuntimeName == "" || p.RuntimeBytes <= 0 {
				t.Errorf("preview(%s/%s) = %+v, want a named runtime with a size", tt.goos, tt.goarch, p)
			}
			continue
		}
		// An unsupported platform must explain itself, not just say no.
		if !strings.Contains(p.Unsupported, tt.goos) || !strings.Contains(p.Unsupported, tt.goarch) {
			t.Errorf("preview(%s/%s).Unsupported = %q, want it to name the platform", tt.goos, tt.goarch, p.Unsupported)
		}
	}
}

func TestPreviewMeasuresThisMachine(t *testing.T) {
	home(t)

	p, err := Preview()
	if err != nil {
		t.Fatal(err)
	}
	if !p.Supported {
		t.Skipf("no prebuilt runtime for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if p.DownloadBytes != p.RuntimeBytes+ModelDownloadBytes {
		t.Errorf("DownloadBytes = %d, want runtime+model = %d with nothing installed",
			p.DownloadBytes, p.RuntimeBytes+ModelDownloadBytes)
	}
	if p.InstallBytes <= ModelDownloadBytes {
		t.Errorf("InstallBytes = %d, want more than the model alone", p.InstallBytes)
	}
	if p.RAMBytes != ModelRAMBytes {
		t.Errorf("RAMBytes = %d, want %d", p.RAMBytes, ModelRAMBytes)
	}
	// These are measured, not constants. A developer machine running the test
	// suite has both.
	if p.FreeDisk <= 0 {
		t.Errorf("FreeDisk = %d, want a real measurement", p.FreeDisk)
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if p.TotalRAM <= 0 {
			t.Errorf("TotalRAM = %d on %s, want a real measurement", p.TotalRAM, runtime.GOOS)
		}
	}
	if p.RuntimePresent || p.ModelPresent {
		t.Error("an empty HITTABLE_HOME reported artifacts as present")
	}
}

func TestPreviewExcludesWhatIsAlreadyThere(t *testing.T) {
	f := newFixture(t)
	if err := f.in.run(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	// The fixture installs a runtime under the real tag but with a fixture
	// hash, so the pinned hash must not match it.
	p, err := Preview()
	if err != nil {
		t.Fatal(err)
	}
	if !p.Supported {
		t.Skip("unsupported platform")
	}
	if p.RuntimePresent {
		t.Error("a runtime extracted from a different archive was reported as present")
	}

	// Now make the marker agree with the pinned hash: that is what a real
	// install looks like.
	marker := filepath.Join(hithome.RuntimeTag(RuntimeTag), markerFile)
	a, _ := runtimeArtifact(runtime.GOOS, runtime.GOARCH)
	if err := os.WriteFile(marker, []byte(a.sum+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ = Preview()
	if !p.RuntimePresent {
		t.Fatal("RuntimePresent is false with a matching hash and a binary in place")
	}
	if p.DownloadBytes != ModelDownloadBytes {
		t.Errorf("DownloadBytes = %d, want only the model (%d)", p.DownloadBytes, ModelDownloadBytes)
	}
}

func TestPlanCheck(t *testing.T) {
	const install = 2 << 30 // 2 GiB, close enough to the real figure

	tests := []struct {
		name     string
		plan     Plan
		wantErr  bool
		wantWarn bool
	}{
		{
			name: "plenty of room",
			plan: Plan{Supported: true, InstallBytes: install, RAMBytes: ModelRAMBytes,
				FreeDisk: 500 << 30, TotalRAM: 32 << 30},
		},
		{
			name: "refuses below 1.3x the install size",
			plan: Plan{Supported: true, InstallBytes: install, RAMBytes: ModelRAMBytes,
				FreeDisk: install, TotalRAM: 32 << 30},
			wantErr: true,
		},
		{
			name: "accepts exactly 1.3x",
			plan: Plan{Supported: true, InstallBytes: install, RAMBytes: ModelRAMBytes,
				FreeDisk: install * 13 / 10, TotalRAM: 32 << 30},
		},
		{
			name: "refuses under 4GB of RAM",
			plan: Plan{Supported: true, InstallBytes: install, RAMBytes: ModelRAMBytes,
				FreeDisk: 500 << 30, TotalRAM: 2 << 30},
			wantErr: true,
		},
		{
			name: "warns between 4 and 8GB",
			plan: Plan{Supported: true, InstallBytes: install, RAMBytes: ModelRAMBytes,
				FreeDisk: 500 << 30, TotalRAM: 6 << 30},
			wantWarn: true,
		},
		{
			// A machine that will not report its memory is not a machine we
			// refuse: an unavailable measurement is not a failed one.
			name: "unknown measurements pass",
			plan: Plan{Supported: true, InstallBytes: install, RAMBytes: ModelRAMBytes},
		},
		{
			name:    "unsupported platform",
			plan:    Plan{Supported: false, Unsupported: "no runtime for plan9/386"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warn, err := tt.plan.Check()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if (warn != "") != tt.wantWarn {
				t.Errorf("warn = %q, wantWarn %v", warn, tt.wantWarn)
			}
		})
	}
}

func TestInstalledIsFalseWithHalfAnInstall(t *testing.T) {
	dir := home(t)

	if Installed() {
		t.Fatal("Installed() is true on an empty home")
	}

	// Runtime only.
	if err := os.MkdirAll(filepath.Dir(ServerPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ServerPath(), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if Installed() {
		t.Error("Installed() is true with a runtime but no model")
	}

	// Model only.
	if err := os.Remove(ServerPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ModelPath(), []byte("weights"), 0o644); err != nil {
		t.Fatal(err)
	}
	if Installed() {
		t.Error("Installed() is true with a model but no runtime")
	}

	// Both.
	if err := os.WriteFile(ServerPath(), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !Installed() {
		t.Error("Installed() is false with both halves in place")
	}
	if !strings.HasPrefix(ServerPath(), dir) || !strings.HasPrefix(ModelPath(), dir) {
		t.Errorf("paths escaped HITTABLE_HOME: %s / %s", ServerPath(), ModelPath())
	}
}

func TestStatusReportsAnInstall(t *testing.T) {
	f := newFixture(t)

	st, err := Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.RuntimeInstalled || st.ModelInstalled || st.ServerRunning {
		t.Fatalf("empty home reported as installed: %+v", st)
	}

	if err := f.in.run(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	st, err = Status()
	if err != nil {
		t.Fatal(err)
	}
	if !st.RuntimeInstalled || st.RuntimeTag != RuntimeTag || st.RuntimeBytes <= 0 {
		t.Errorf("runtime: %+v", st)
	}
	if !st.ModelInstalled || st.ModelFile != ModelFileName || st.ModelBytes != int64(len(f.mdBody)) {
		t.Errorf("model: %+v", st)
	}
	if st.TotalBytes < st.RuntimeBytes+st.ModelBytes {
		t.Errorf("TotalBytes = %d, less than its own parts", st.TotalBytes)
	}
	if st.Endpoint != "" {
		t.Errorf("Endpoint = %q with nothing configured", st.Endpoint)
	}
}

func TestStatusReportsAConfiguredEndpoint(t *testing.T) {
	home(t)
	t.Setenv("HITTABLE_AI_ENDPOINT", "http://10.0.0.2:9090")

	st, err := Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Endpoint != "http://10.0.0.2:9090" {
		t.Errorf("Endpoint = %q, want the configured one", st.Endpoint)
	}
}

func TestRemoveFreesTheDiskAndKeepsTheConfig(t *testing.T) {
	f := newFixture(t)
	if err := f.in.run(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

	cfg := hithome.ConfigPath()
	if err := os.WriteFile(cfg, []byte(`{"ai":{"enabled":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := Status()
	if err != nil {
		t.Fatal(err)
	}
	freed, err := Remove()
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if freed < before.RuntimeBytes+before.ModelBytes {
		t.Errorf("freed = %d, want at least the %d it was holding",
			freed, before.RuntimeBytes+before.ModelBytes)
	}
	if Installed() {
		t.Error("Installed() is still true after Remove")
	}
	// Settings are not what `disable` was asked to delete.
	if _, err := os.Stat(cfg); err != nil {
		t.Errorf("Remove deleted config.json: %v", err)
	}
	for _, dir := range []string{hithome.Runtime(), hithome.Models(), hithome.Run(), hithome.Tmp(), hithome.Logs()} {
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("%s survived Remove", dir)
		}
	}

	// Removing nothing is not an error, and frees nothing.
	freed, err = Remove()
	if err != nil || freed != 0 {
		t.Errorf("second Remove = %d, %v; want 0, nil", freed, err)
	}
}

func TestArtifactTableIsWellFormed(t *testing.T) {
	// A wrong hash is indistinguishable from a corrupt download, and the error
	// it produces would blame the network. Shape is all a test can check, so it
	// checks all of it.
	for key, a := range runtimeAssets {
		if !strings.HasPrefix(a.File, "llama-"+RuntimeTag+"-bin-") {
			t.Errorf("%s: file %q is not from tag %s", key, a.File, RuntimeTag)
		}
		if len(a.SHA) != 64 {
			t.Errorf("%s: sha %q is not a sha256", key, a.SHA)
		}
		if a.Bytes <= 0 {
			t.Errorf("%s: bytes = %d", key, a.Bytes)
		}
		wantZip := strings.HasPrefix(key, "windows/")
		if gotZip := strings.HasSuffix(a.File, ".zip"); gotZip != wantZip {
			t.Errorf("%s: %q — only Windows ships .zip", key, a.File)
		}
	}
	if len(ModelSHA256) != 64 {
		t.Errorf("model sha %q is not a sha256", ModelSHA256)
	}
	// Pinned to a commit, not a moving branch, or the hash goes stale silently.
	if strings.Contains(modelOrigin, "/main/") {
		t.Errorf("model origin %q is not pinned to an immutable commit", modelOrigin)
	}

	rt, err := runtimeArtifact("plan9", "386")
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("runtimeArtifact(plan9/386) = %+v, %v; want ErrUnsupported", rt, err)
	}
}

func TestArtifactKind(t *testing.T) {
	tests := map[string]string{
		"llama-b11120-bin-macos-arm64.tar.gz":  "tar.gz",
		"llama-b11120-bin-ubuntu-x64.tar.gz":   "tar.gz",
		"llama-b11120-bin-win-cpu-x64.zip":     "zip",
		"qwen2.5-coder-3b-instruct-q4_k_m.tgz": "tar.gz",
	}
	for file, want := range tests {
		if got := (artifact{file: file}).kind(); got != want {
			t.Errorf("kind(%q) = %q, want %q", file, got, want)
		}
	}
}
