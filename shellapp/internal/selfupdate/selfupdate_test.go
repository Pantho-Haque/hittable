package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// release serves one tag the way the release workflow publishes it.
type release struct {
	tag     string
	asset   string
	payload []byte
	sum     string
	// corrupt serves a body that does not match the published checksum.
	corrupt bool
	// noSums omits checksums.txt entirely.
	noSums bool
	hits   map[string]int
}

func targz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// withExecutable points Apply at path for the duration of a test, so it
// replaces a fixture rather than the running test binary.
func withExecutable(t *testing.T, path string) {
	t.Helper()
	prev := executable
	executable = func() (string, error) { return path, nil }
	t.Cleanup(func() { executable = prev })
}

func serve(t *testing.T, r *release) (*httptest.Server, Config) {
	t.Helper()
	r.hits = map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.hits[req.URL.Path]++
		switch {
		case strings.HasSuffix(req.URL.Path, "/releases/latest"):
			fmt.Fprintf(w, `{"tag_name": %q, "name": "whatever"}`, r.tag)
		case strings.HasSuffix(req.URL.Path, "/checksums.txt"):
			if r.noSums {
				http.NotFound(w, req)
				return
			}
			fmt.Fprintf(w, "%s  %s\n", r.sum, r.asset)
		case strings.HasSuffix(req.URL.Path, r.asset):
			body := r.payload
			if r.corrupt {
				body = append(append([]byte{}, body...), 'x')
			}
			w.Write(body)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, Config{
		Repo: "acme/tool", Current: "v1.0.0", Site: srv.URL, API: srv.URL,
		GOOS: "linux", GOARCH: "amd64", Client: srv.Client(),
	}
}

func newRelease(t *testing.T, tag string, body []byte) *release {
	t.Helper()
	asset := AssetName(tag, "linux", "amd64")
	pay := targz(t, binName, body)
	return &release{tag: tag, asset: asset, payload: pay, sum: sum(pay)}
}

func TestLatest(t *testing.T) {
	_, cfg := serve(t, newRelease(t, "shellapp-v2.0.0", []byte("#!/bin/sh\n")))
	got, err := Latest(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got != "shellapp-v2.0.0" {
		t.Errorf("Latest = %q", got)
	}
}

// The whole point: the binary on disk is replaced by the downloaded one.
func TestApplyReplacesTheExecutable(t *testing.T) {
	r := newRelease(t, "shellapp-v2.0.0", []byte("NEW BINARY"))
	_, cfg := serve(t, r)

	dir := t.TempDir()
	target := filepath.Join(dir, "hittable")
	if err := os.WriteFile(target, []byte("OLD BINARY"), 0o755); err != nil {
		t.Fatal(err)
	}
	withExecutable(t, target)

	got, err := Apply(context.Background(), cfg, r.tag, nil)
	if err != nil {
		t.Fatal(err)
	}
	// macOS /var is a symlink to /private/var and Apply resolves it, which is
	// what you want when replacing a binary through one.
	want, _ := filepath.EvalSymlinks(target)
	if got != want {
		t.Errorf("Apply returned %q, want %q", got, want)
	}
	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "NEW BINARY" {
		t.Errorf("binary content = %q, want the downloaded one", b)
	}
	if fi, _ := os.Stat(target); fi.Mode().Perm()&0o111 == 0 {
		t.Error("the replacement is not executable")
	}
}

// A corrupted download is code we were about to execute. It must not land, and
// the old binary must survive.
func TestApplyRefusesACorruptedDownload(t *testing.T) {
	r := newRelease(t, "shellapp-v2.0.0", []byte("NEW BINARY"))
	r.corrupt = true
	_, cfg := serve(t, r)

	dir := t.TempDir()
	target := filepath.Join(dir, "hittable")
	os.WriteFile(target, []byte("OLD BINARY"), 0o755)
	withExecutable(t, target)

	if _, err := Apply(context.Background(), cfg, r.tag, nil); err == nil {
		t.Fatal("Apply accepted a payload that failed its checksum")
	} else if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("err = %v, want a checksum mismatch", err)
	}
	if b, _ := os.ReadFile(target); string(b) != "OLD BINARY" {
		t.Errorf("the old binary was damaged: %q", b)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".hittable*")); len(left) != 0 {
		t.Errorf("staging files left behind: %v", left)
	}
}

// No checksums means nothing to verify against, so there is nothing to trust.
func TestApplyRefusesWhenNoChecksumsArePublished(t *testing.T) {
	r := newRelease(t, "shellapp-v2.0.0", []byte("NEW"))
	r.noSums = true
	_, cfg := serve(t, r)

	dir := t.TempDir()
	target := filepath.Join(dir, "hittable")
	os.WriteFile(target, []byte("OLD"), 0o755)
	withExecutable(t, target)

	if _, err := Apply(context.Background(), cfg, r.tag, nil); err == nil {
		t.Fatal("Apply installed an unverified artifact")
	}
	if b, _ := os.ReadFile(target); string(b) != "OLD" {
		t.Error("the old binary was replaced anyway")
	}
}

func TestApplyReportsAMissingBuildForThisPlatform(t *testing.T) {
	r := newRelease(t, "shellapp-v2.0.0", []byte("NEW"))
	_, cfg := serve(t, r)
	cfg.GOARCH = "riscv64" // nothing published for it

	dir := t.TempDir()
	target := filepath.Join(dir, "hittable")
	os.WriteFile(target, []byte("OLD"), 0o755)
	withExecutable(t, target)

	_, err := Apply(context.Background(), cfg, r.tag, nil)
	if err == nil || !strings.Contains(err.Error(), "riscv64") {
		t.Errorf("err = %v, want it to name the missing platform", err)
	}
}

// Refuse before downloading 25MB, not after.
func TestApplyChecksWritabilityFirst(t *testing.T) {
	r := newRelease(t, "shellapp-v2.0.0", []byte("NEW"))
	_, cfg := serve(t, r)

	dir := t.TempDir()
	target := filepath.Join(dir, "hittable")
	os.WriteFile(target, []byte("OLD"), 0o755)
	withExecutable(t, target)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Skip("cannot make the directory read-only here")
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if os.Geteuid() == 0 {
		t.Skip("running as root, permissions do not apply")
	}

	_, err := Apply(context.Background(), cfg, r.tag, nil)
	if err == nil || !strings.Contains(err.Error(), "not writable") {
		t.Fatalf("err = %v, want a writability error", err)
	}
	if r.hits["/acme/tool/releases/download/"+r.tag+"/"+r.asset] != 0 {
		t.Error("it downloaded the asset before checking it could install it")
	}
}

// An archive is untrusted input even when we published it.
func TestExtractRejectsAPathEscape(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("pwned")
	tw.WriteHeader(&tar.Header{Name: "../../evil", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
	tw.Write(body)
	tw.Close()
	gz.Close()

	archive := filepath.Join(dir, "a.tar.gz")
	os.WriteFile(archive, buf.Bytes(), 0o644)

	if err := extract(archive, filepath.Join(dir, "into"), "linux"); err == nil {
		t.Fatal("extract accepted an entry that escapes the destination")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil")); err == nil {
		t.Fatal("an entry was written outside the destination")
	}
}

func TestAssetName(t *testing.T) {
	tests := []struct {
		goos, goarch, want string
	}{
		{"darwin", "arm64", "hittable_v1_darwin_arm64.tar.gz"},
		{"linux", "amd64", "hittable_v1_linux_amd64.tar.gz"},
		{"windows", "amd64", "hittable_v1_windows_amd64.zip"},
	}
	for _, tt := range tests {
		if got := AssetName("v1", tt.goos, tt.goarch); got != tt.want {
			t.Errorf("AssetName(%s/%s) = %q, want %q", tt.goos, tt.goarch, got, tt.want)
		}
	}
}

func TestSumFor(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.txt")
	os.WriteFile(p, []byte("aaa  hittable_v1_linux_amd64.tar.gz\nbbb  other.zip\n"), 0o644)
	if got, err := sumFor(p, "hittable_v1_linux_amd64.tar.gz"); err != nil || got != "aaa" {
		t.Errorf("sumFor = %q, %v", got, err)
	}
	if _, err := sumFor(p, "missing.tar.gz"); err == nil {
		t.Error("sumFor accepted an asset that is not listed")
	}
}
