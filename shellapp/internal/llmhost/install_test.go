package llmhost

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hittable/shellapp/internal/hithome"
)

// Every test here points HITTABLE_HOME at a temp directory and every byte comes
// from an httptest origin. Nothing in this package's test suite touches the
// network, downloads a model, or writes outside t.TempDir().

func home(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(hithome.EnvHome, dir)
	if err := hithome.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	return dir
}

// --- archive fixtures -------------------------------------------------------

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range sortedKeys(files) {
		body := files[name]
		if err := tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range sortedKeys(files) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(files[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Insertion sort: the maps here have a handful of entries.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func sum256(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// runtimeFixture is a plausible release archive: the binary nested under
// build/bin the way the real macOS and Linux tarballs nest it, plus a library
// beside it.
func runtimeFixture(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"build/bin/" + serverBinName(runtime.GOOS): "#!/bin/sh\nexit 0\n",
		"build/bin/libllama.dylib":                 "not really a library",
		"build/bin/README.md":                      "docs",
	}
}

// --- test origin ------------------------------------------------------------

// origin serves fixed bodies, honours Range the way GitHub and Hugging Face do,
// and can be told to ignore Range so the 200-instead-of-206 path is reachable.
type origin struct {
	srv *httptest.Server

	mu      sync.Mutex
	files   map[string][]byte
	noRange map[string]bool
	ranges  map[string][]string
	hits    map[string]int
	fail    map[string]*failPlan
}

// failPlan scripts the first few attempts at a path to fail, so the retry loop
// is exercised against something that behaves like a flaky network rather than
// a mock.
type failPlan struct {
	remaining  int
	status     int    // 0 means abort the connection mid-body instead
	afterBytes int    // bytes to deliver before aborting
	retryAfter string // Retry-After header, when status is set
}

func newOrigin(t *testing.T, files map[string][]byte) *origin {
	t.Helper()
	o := &origin{
		files:   files,
		noRange: map[string]bool{},
		ranges:  map[string][]string{},
		hits:    map[string]int{},
		fail:    map[string]*failPlan{},
	}
	o.srv = httptest.NewServer(http.HandlerFunc(o.serve))
	t.Cleanup(o.srv.Close)
	return o
}

func (o *origin) serve(w http.ResponseWriter, r *http.Request) {
	o.mu.Lock()
	body, ok := o.files[r.URL.Path]
	o.hits[r.URL.Path]++
	rh := r.Header.Get("Range")
	if rh != "" {
		o.ranges[r.URL.Path] = append(o.ranges[r.URL.Path], rh)
	}
	noRange := o.noRange[r.URL.Path]
	var plan failPlan
	failing := false
	if fp := o.fail[r.URL.Path]; fp != nil && fp.remaining > 0 {
		fp.remaining--
		plan, failing = *fp, true
	}
	o.mu.Unlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	if failing && plan.status != 0 {
		if plan.retryAfter != "" {
			w.Header().Set("Retry-After", plan.retryAfter)
		}
		w.WriteHeader(plan.status)
		return
	}

	start := 0
	if rh != "" && !noRange {
		if _, err := fmt.Sscanf(rh, "bytes=%d-", &start); err != nil || start > len(body) {
			start = 0
		}
	}
	if start > 0 {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(body)-1, len(body)))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)-start))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
	}

	// Written in flushed chunks so the client sees a body that genuinely
	// arrives over time rather than one atomic write, which is what makes an
	// in-flight observation possible.
	const chunk = 16 << 10
	rest := body[start:]

	if failing {
		// Deliver a prefix, then drop the connection the way a real reset does.
		// The bytes already written stay in the .part, which is the whole point.
		n := min(plan.afterBytes, len(rest))
		_, _ = w.Write(rest[:n])
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		panic(http.ErrAbortHandler)
	}

	for len(rest) > 0 {
		n := min(chunk, len(rest))
		if _, err := w.Write(rest[:n]); err != nil {
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		rest = rest[n:]
	}
}

func (o *origin) hitCount(path string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.hits[path]
}

func (o *origin) rangeHeaders(path string) []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.ranges[path]...)
}

// --- installer under test ---------------------------------------------------

const (
	rtPath    = "/runtime.tar.gz"
	modelPath = "/model.gguf"
)

type fixture struct {
	in       *installer
	o        *origin
	rtBody   []byte
	mdBody   []byte
	postDir  string
	postBin  string
	postRuns int
}

func newFixture(t *testing.T, opts ...func(*fixture)) *fixture {
	t.Helper()
	home(t)

	f := &fixture{
		rtBody: tarGz(t, runtimeFixture(t)),
		// Big enough that it cannot arrive in a single read, so a download in
		// progress is genuinely observable.
		mdBody: bytes.Repeat([]byte("GGUF pretend weights; "), 12000),
	}
	for _, opt := range opts {
		opt(f)
	}

	f.o = newOrigin(t, map[string][]byte{rtPath: f.rtBody, modelPath: f.mdBody})
	f.in = &installer{
		runtime: artifact{
			label: "runtime", url: f.o.srv.URL + rtPath,
			file: "runtime.tar.gz", bytes: int64(len(f.rtBody)), sum: sum256(f.rtBody),
		},
		model: artifact{
			label: "model", url: f.o.srv.URL + modelPath,
			file: ModelFileName, bytes: int64(len(f.mdBody)), sum: sum256(f.mdBody),
		},
		client: f.o.srv.Client(),
		goos:   runtime.GOOS,
		// Real timing is asserted separately, in TestDefaultBackoff.
		backoff: func(int, time.Duration) time.Duration { return 0 },
		post: func(dir, bin string) {
			f.postDir, f.postBin = dir, bin
			f.postRuns++
		},
	}
	return f
}

func TestInstallLandsBothArtifacts(t *testing.T) {
	f := newFixture(t)
	if err := f.in.run(context.Background(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !Installed() {
		t.Fatal("Installed() is false after a successful install")
	}
	if got, err := os.ReadFile(ModelPath()); err != nil || !bytes.Equal(got, f.mdBody) {
		t.Fatalf("model content: %v / %q", err, got)
	}
	// The archive nests under build/bin; the binary must end up at the top.
	if _, err := os.Stat(ServerPath()); err != nil {
		t.Fatalf("server binary not at %s: %v", ServerPath(), err)
	}
	if _, err := os.Stat(filepath.Join(hithome.RuntimeTag(RuntimeTag), "libllama.dylib")); err != nil {
		t.Fatalf("library was not flattened alongside the binary: %v", err)
	}
	if _, err := os.Stat(filepath.Join(hithome.RuntimeTag(RuntimeTag), "build")); err == nil {
		t.Error("the nested build/ directory survived flattening")
	}

	cur, err := os.ReadFile(hithome.RuntimeCurrent())
	if err != nil || strings.TrimSpace(string(cur)) != RuntimeTag {
		t.Fatalf("runtime/current = %q, %v", cur, err)
	}
	if f.postRuns != 1 || f.postBin == "" {
		t.Fatalf("post-processing ran %d times, bin=%q", f.postRuns, f.postBin)
	}
	// Post-processing must run on the staging tree, before the rename — signing
	// the live directory would mean a window where it is visible and unsigned.
	if !strings.Contains(f.postDir, ".tmp-") {
		t.Errorf("post-processed %q, expected the staging directory", f.postDir)
	}
}

func TestInstallIsAtomic(t *testing.T) {
	f := newFixture(t)

	// Progress fires from inside the copy loop, so a partial report is a
	// guaranteed observation of a download that has not finished.
	var partial int
	err := f.in.run(context.Background(), func(label string, done, total int64) {
		if done >= total {
			return
		}
		partial++
		name := "runtime.tar.gz"
		if label == "model" {
			name = ModelFileName
			if _, err := os.Stat(ModelPath()); err == nil {
				t.Error("the model exists under its real name while still downloading")
			}
		} else if _, err := os.Stat(hithome.RuntimeTag(RuntimeTag)); err == nil {
			t.Error("runtime/<tag> exists while its archive is still downloading")
		}
		if _, err := os.Stat(filepath.Join(hithome.Tmp(), name+partSuffix)); err != nil {
			t.Errorf("no in-progress .part file for %s: %v", label, err)
		}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if partial == 0 {
		t.Fatal("no partial progress was observed; the assertions above never ran")
	}
	// And nothing is left behind once it is done.
	parts, _ := os.ReadDir(hithome.Tmp())
	if len(parts) != 0 {
		t.Errorf("tmp/ still holds %d entries after a clean install", len(parts))
	}
}

func TestInstallRefusesOnChecksumMismatch(t *testing.T) {
	f := newFixture(t)
	f.in.runtime.sum = strings.Repeat("0", 64)

	err := f.in.run(context.Background(), nil)
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
	// The message has to name both hashes or it cannot be acted on.
	if !strings.Contains(err.Error(), f.in.runtime.sum) || !strings.Contains(err.Error(), sum256(f.rtBody)) {
		t.Errorf("error does not report expected vs actual: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(hithome.Tmp(), "runtime.tar.gz"+partSuffix)); statErr == nil {
		t.Error("the unverified download was kept")
	}
	if _, statErr := os.Stat(hithome.RuntimeTag(RuntimeTag)); statErr == nil {
		t.Error("an unverified archive was installed")
	}
	if Installed() {
		t.Error("Installed() is true after a refused install")
	}
}

func TestDownloadResumesFromPart(t *testing.T) {
	f := newFixture(t)

	// Leave a correct prefix of the model behind, as an interrupted run would.
	const prefix = 20
	part := filepath.Join(hithome.Tmp(), ModelFileName+partSuffix)
	if err := os.WriteFile(part, f.mdBody[:prefix], 0o644); err != nil {
		t.Fatal(err)
	}

	if err := f.in.run(context.Background(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := f.o.rangeHeaders(modelPath); len(got) != 1 || got[0] != fmt.Sprintf("bytes=%d-", prefix) {
		t.Fatalf("Range headers = %v, want one bytes=%d-", got, prefix)
	}
	if got, _ := os.ReadFile(ModelPath()); !bytes.Equal(got, f.mdBody) {
		t.Fatalf("resumed content = %q, want %q", got, f.mdBody)
	}
}

func TestDownloadRestartsWhenServerIgnoresRange(t *testing.T) {
	f := newFixture(t)
	f.o.mu.Lock()
	f.o.noRange[modelPath] = true
	f.o.mu.Unlock()

	// Deliberately wrong bytes: appending a 200 response to these would produce
	// a file that only the hash could catch. Truncating is what must happen.
	part := filepath.Join(hithome.Tmp(), ModelFileName+partSuffix)
	if err := os.WriteFile(part, []byte("XXXXXXXXXXXXXXXXXXXX"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := f.in.run(context.Background(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, _ := os.ReadFile(ModelPath()); !bytes.Equal(got, f.mdBody) {
		t.Fatalf("content = %q, want %q", got, f.mdBody)
	}
}

func TestInstallSkipsWhatIsAlreadyThere(t *testing.T) {
	f := newFixture(t)
	if err := f.in.run(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	before := f.o.hitCount(rtPath) + f.o.hitCount(modelPath)

	if err := f.in.run(context.Background(), nil); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if after := f.o.hitCount(rtPath) + f.o.hitCount(modelPath); after != before {
		t.Fatalf("a second install made %d more requests; it should make none", after-before)
	}
}

func TestInstallCollectsStaleState(t *testing.T) {
	f := newFixture(t)

	stale := filepath.Join(hithome.Runtime(), ".tmp-b00001")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	oldPart := filepath.Join(hithome.Tmp(), "some-older-model.gguf"+partSuffix)
	if err := os.WriteFile(oldPart, []byte("abandoned"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A part for an artifact we still want is not garbage — it is the resume.
	livePart := filepath.Join(hithome.Tmp(), ModelFileName+partSuffix)
	if err := os.WriteFile(livePart, f.mdBody[:10], 0o644); err != nil {
		t.Fatal(err)
	}

	f.in.gc()

	if _, err := os.Stat(stale); err == nil {
		t.Error("a stale staging directory survived gc")
	}
	if _, err := os.Stat(oldPart); err == nil {
		t.Error("a partial download for an artifact we no longer want survived gc")
	}
	if _, err := os.Stat(livePart); err != nil {
		t.Errorf("gc deleted the partial download it was supposed to resume: %v", err)
	}
}

func TestInstallSurvivesAnInterruptedExtract(t *testing.T) {
	f := newFixture(t)

	// Exactly what a crash during extraction leaves behind.
	stage := filepath.Join(hithome.Runtime(), ".tmp-"+RuntimeTag)
	if err := os.MkdirAll(filepath.Join(stage, "build", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "build", "bin", "half"), []byte("truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if Installed() {
		t.Fatal("a staging directory is being read as an install")
	}

	if err := f.in.run(context.Background(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !Installed() {
		t.Fatal("the next run did not recover")
	}
	entries, _ := os.ReadDir(hithome.Runtime())
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("staging directory %s survived a completed install", e.Name())
		}
	}
	if _, err := os.Stat(filepath.Join(hithome.RuntimeTag(RuntimeTag), "half")); err == nil {
		t.Error("debris from the interrupted extract was installed")
	}
}

func TestInstallHandlesZipArchives(t *testing.T) {
	f := newFixture(t, func(f *fixture) {
		// Windows ships .zip; everything else ships .tar.gz. Both paths matter.
		f.rtBody = zipArchive(t, map[string]string{
			serverBinName(runtime.GOOS): "#!/bin/sh\nexit 0\n",
			"ggml.dll":                  "library",
		})
	})
	f.in.runtime.file = "runtime.zip"
	if f.in.runtime.kind() != "zip" {
		t.Fatalf("kind = %q, want zip", f.in.runtime.kind())
	}

	if err := f.in.run(context.Background(), nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(ServerPath()); err != nil {
		t.Fatalf("zip install did not produce a binary: %v", err)
	}
}

func TestExtractRejectsPathEscapes(t *testing.T) {
	dir := t.TempDir()
	escapes := map[string]string{
		"../../evil":            "pwned",
		"build/bin/../../../up": "pwned",
	}

	for name, body := range escapes {
		t.Run("tar "+name, func(t *testing.T) {
			src := filepath.Join(t.TempDir(), "a.tar.gz")
			if err := os.WriteFile(src, tarGz(t, map[string]string{name: body}), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := extractArchive("tar.gz", src, dir); !errors.Is(err, ErrUnsafeArchive) {
				t.Fatalf("err = %v, want ErrUnsafeArchive", err)
			}
		})
		t.Run("zip "+name, func(t *testing.T) {
			src := filepath.Join(t.TempDir(), "a.zip")
			if err := os.WriteFile(src, zipArchive(t, map[string]string{name: body}), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := extractArchive("zip", src, dir); !errors.Is(err, ErrUnsafeArchive) {
				t.Fatalf("err = %v, want ErrUnsafeArchive", err)
			}
		})
	}

	// And nothing escaped while we were checking.
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil")); err == nil {
		t.Fatal("an entry escaped the destination")
	}
}

func TestSafePath(t *testing.T) {
	dest := filepath.FromSlash("/dest")
	tests := []struct {
		name string
		ok   bool
	}{
		{"llama-server", true},
		{"build/bin/llama-server", true},
		{"./build/bin/llama-server", true},
		{"build/bin/", true},
		{"../evil", false},
		{"build/../../evil", false},
		{"/etc/passwd", false},
		{"", false},
		{".", false},
		{"..", false},
	}
	for _, tt := range tests {
		got, err := safePath(dest, tt.name)
		if (err == nil) != tt.ok {
			t.Errorf("safePath(%q) err = %v, want ok=%v", tt.name, err, tt.ok)
			continue
		}
		if tt.ok && !strings.HasPrefix(got, dest) {
			t.Errorf("safePath(%q) = %q, outside %q", tt.name, got, dest)
		}
	}
}

func TestLinkStaysInside(t *testing.T) {
	tests := []struct {
		entry, link string
		ok          bool
	}{
		{"build/bin/libllama.so", "libllama.so.1", true},
		{"build/bin/libllama.so", "../lib/libllama.so.1", true},
		{"build/bin/libllama.so", "../../../etc/passwd", false},
		{"libllama.so", "../outside", false},
		{"build/bin/x", "/etc/passwd", false},
		{"build/bin/x", "", false},
	}
	for _, tt := range tests {
		if got := linkStaysInside(tt.entry, tt.link); got != tt.ok {
			t.Errorf("linkStaysInside(%q, %q) = %v, want %v", tt.entry, tt.link, got, tt.ok)
		}
	}
}

func TestProgressReachesTotal(t *testing.T) {
	f := newFixture(t)

	var mu sync.Mutex
	last := map[string][2]int64{}
	err := f.in.run(context.Background(), func(label string, done, total int64) {
		mu.Lock()
		defer mu.Unlock()
		last[label] = [2]int64{done, total}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"runtime", "model"} {
		got, ok := last[label]
		if !ok {
			t.Fatalf("no progress was reported for %q", label)
		}
		if got[0] != got[1] || got[1] == 0 {
			t.Errorf("%s finished at %d/%d", label, got[0], got[1])
		}
	}
}

// --- retry ------------------------------------------------------------------

// failAfter scripts the next n attempts at path to abort mid-body, having
// delivered afterBytes first.
func (o *origin) failAfter(path string, n, afterBytes int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.fail[path] = &failPlan{remaining: n, afterBytes: afterBytes}
}

func (o *origin) failStatus(path string, n, status int, retryAfter string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.fail[path] = &failPlan{remaining: n, status: status, retryAfter: retryAfter}
}

func TestDownloadRetriesAndAccumulatesBytes(t *testing.T) {
	f := newFixture(t)
	const chunkPerAttempt = 40000
	f.o.failAfter(modelPath, 2, chunkPerAttempt)

	var retries []string
	err := f.in.run(context.Background(), func(label string, _, _ int64) {
		if strings.Contains(label, "retrying") {
			retries = append(retries, label)
		}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// The whole point: each attempt resumes where the last one died rather than
	// starting over, so the Range offsets climb.
	got := f.o.rangeHeaders(modelPath)
	want := []string{
		fmt.Sprintf("bytes=%d-", chunkPerAttempt),
		fmt.Sprintf("bytes=%d-", chunkPerAttempt*2),
	}
	if len(got) != len(want) {
		t.Fatalf("Range headers = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("attempt %d sent %q, want %q — progress restarted instead of resuming", i+2, got[i], want[i])
		}
	}

	if got, _ := os.ReadFile(ModelPath()); !bytes.Equal(got, f.mdBody) {
		t.Error("the reassembled file does not match the original")
	}
	// The CLI has to be able to say "retrying" rather than appear to stall.
	if len(retries) != 2 {
		t.Errorf("Progress saw %d retry labels %v, want 2", len(retries), retries)
	}
	for i, label := range retries {
		if !strings.Contains(label, fmt.Sprintf("%d/%d", i+2, f.in.attempts)) &&
			!strings.Contains(label, fmt.Sprintf("%d/%d", i+2, maxDownloadAttempts)) {
			t.Errorf("retry label %q does not report attempt %d of the total", label, i+2)
		}
	}
}

func TestDownloadRetriesServerErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		retryAfter string
		wantRetry  bool
	}{
		{"500", http.StatusInternalServerError, "", true},
		{"502", http.StatusBadGateway, "", true},
		{"503 with Retry-After", http.StatusServiceUnavailable, "1", true},
		{"429 with Retry-After", http.StatusTooManyRequests, "2", true},
		{"429 bare", http.StatusTooManyRequests, "", true},
		{"404", http.StatusNotFound, "", false},
		{"403", http.StatusForbidden, "", false},
		{"401", http.StatusUnauthorized, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			var hints []time.Duration
			f.in.backoff = func(_ int, hint time.Duration) time.Duration {
				hints = append(hints, hint)
				return 0
			}
			f.o.failStatus(modelPath, 1, tt.status, tt.retryAfter)

			err := f.in.run(context.Background(), nil)
			if tt.wantRetry {
				if err != nil {
					t.Fatalf("a retryable %d was not retried: %v", tt.status, err)
				}
				if got, _ := os.ReadFile(ModelPath()); !bytes.Equal(got, f.mdBody) {
					t.Error("content mismatch after a successful retry")
				}
				// Retry-After is the server telling us how long to wait; it has
				// to reach the backoff rather than being ignored.
				if tt.retryAfter != "" {
					if len(hints) != 1 || hints[0] <= 0 {
						t.Errorf("Retry-After %q produced hints %v, want one positive duration", tt.retryAfter, hints)
					}
				}
				return
			}

			if err == nil {
				t.Fatalf("a %d was treated as retryable and eventually succeeded", tt.status)
			}
			if n := f.o.hitCount(modelPath); n != 1 {
				t.Errorf("a %d was requested %d times, want exactly 1 — retrying it is pointless", tt.status, n)
			}
		})
	}
}

func TestDownloadGivesUpButKeepsTheProgress(t *testing.T) {
	f := newFixture(t)
	const chunkPerAttempt = 10000
	f.in.attempts = 3
	f.o.failAfter(modelPath, 99, chunkPerAttempt) // never succeeds

	err := f.in.run(context.Background(), nil)
	if err == nil {
		t.Fatal("run succeeded against an origin that always fails")
	}
	if n := f.o.hitCount(modelPath); n != 3 {
		t.Errorf("made %d attempts, want the configured 3", n)
	}
	// The error has to tell the user the work is not lost.
	if !strings.Contains(err.Error(), "resume") {
		t.Errorf("err = %v, want it to say a rerun resumes", err)
	}

	// Giving up keeps the .part, so a later rerun picks up from there.
	part := filepath.Join(hithome.Tmp(), ModelFileName+partSuffix)
	fi, statErr := os.Stat(part)
	if statErr != nil {
		t.Fatalf("the partial download was discarded: %v", statErr)
	}
	if fi.Size() != chunkPerAttempt*3 {
		t.Errorf(".part is %d bytes, want the %d accumulated across 3 attempts", fi.Size(), chunkPerAttempt*3)
	}

	// And a rerun against a healthy origin finishes the job.
	f.o.mu.Lock()
	f.o.fail[modelPath] = nil
	f.o.mu.Unlock()
	if err := f.in.run(context.Background(), nil); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if got, _ := os.ReadFile(ModelPath()); !bytes.Equal(got, f.mdBody) {
		t.Error("the resumed file does not match the original")
	}
}

func TestDownloadDoesNotRetryAChecksumMismatch(t *testing.T) {
	f := newFixture(t)
	f.in.model.sum = strings.Repeat("a", 64)

	err := f.in.run(context.Background(), nil)
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
	// Waiting cannot turn a wrong hash into a right one. Retrying would be four
	// more pointless 2GB downloads.
	if n := f.o.hitCount(modelPath); n != 1 {
		t.Errorf("a checksum mismatch was downloaded %d times, want exactly 1", n)
	}
}

func TestCancelDuringTransferStopsImmediately(t *testing.T) {
	f := newFixture(t)
	f.o.failAfter(modelPath, 99, 1000)
	f.in.backoff = func(int, time.Duration) time.Duration { return time.Hour }

	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	err := f.in.run(ctx, func(label string, _, _ int64) {
		if label == "model" {
			once.Do(cancel)
		}
	})

	// Cancellation is the user pressing ctrl-c. It must surface as such and
	// must not be retried — note the backoff above would hang for an hour.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if n := f.o.hitCount(modelPath); n > 1 {
		t.Errorf("a cancelled download was retried %d times", n)
	}
	if _, err := os.Stat(ModelPath()); err == nil {
		t.Error("a cancelled download was installed")
	}
}

// The other half of cancellation: ctrl-c while the retry loop is sitting in
// its backoff, which is exactly when a user watching "retrying 2/5" gives up.
func TestCancelDuringBackoffStopsImmediately(t *testing.T) {
	f := newFixture(t)
	f.o.failAfter(modelPath, 99, 1000)
	// Long enough that a missing ctx guard shows up as a hung test rather than
	// a slow one.
	f.in.backoff = func(int, time.Duration) time.Duration { return time.Hour }

	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once

	done := make(chan error, 1)
	go func() {
		done <- f.in.run(ctx, func(label string, _, _ int64) {
			if strings.Contains(label, "retrying") {
				once.Do(cancel)
			}
		})
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("cancelling during the backoff did not interrupt it")
	}
	if _, err := os.Stat(ModelPath()); err == nil {
		t.Error("a cancelled download was installed")
	}
}

func TestDefaultBackoff(t *testing.T) {
	// Doubling, jittered over the top half, capped — and a server's own
	// Retry-After always wins.
	for attempt, want := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second} {
		for i := 0; i < 50; i++ {
			got := defaultBackoff(attempt, 0)
			if got < want/2 || got > want {
				t.Fatalf("defaultBackoff(%d) = %v, want within [%v, %v]", attempt, got, want/2, want)
			}
		}
	}
	if got := defaultBackoff(20, 0); got > maxBackoff {
		t.Errorf("defaultBackoff(20) = %v, want capped at %v", got, maxBackoff)
	}
	if got := defaultBackoff(1, 5*time.Second); got != 5*time.Second {
		t.Errorf("defaultBackoff with a Retry-After hint = %v, want 5s", got)
	}
	if got := defaultBackoff(1, time.Hour); got != maxBackoff {
		t.Errorf("a Retry-After of an hour = %v, want it capped at %v", got, maxBackoff)
	}
}

func TestRetryAfterHeader(t *testing.T) {
	tests := []struct {
		value string
		want  time.Duration
	}{
		{"", 0},
		{"2", 2 * time.Second},
		{"0", 0},
		{"  7 ", 7 * time.Second},
		{"-1", 0},
		{"garbage", 0},
		{time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat), 0},
	}
	for _, tt := range tests {
		h := http.Header{}
		if tt.value != "" {
			h.Set("Retry-After", tt.value)
		}
		if got := retryAfter(h); got != tt.want {
			t.Errorf("retryAfter(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
	// An HTTP-date in the future resolves to a positive wait.
	h := http.Header{}
	h.Set("Retry-After", time.Now().Add(30*time.Second).UTC().Format(http.TimeFormat))
	if got := retryAfter(h); got <= 0 || got > 31*time.Second {
		t.Errorf("retryAfter(future date) = %v, want ~30s", got)
	}
}
