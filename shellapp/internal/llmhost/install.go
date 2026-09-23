package llmhost

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hittable/shellapp/internal/hithome"
)

// Progress reports download and install progress. label names the artifact
// being worked on and changes when the installer moves to the next one; done
// and total are bytes.
type Progress func(label string, done, total int64)

// Install downloads, verifies and installs everything the local model needs.
// It is resumable and idempotent: an artifact already present and matching its
// pinned hash is skipped, and an interrupted run continues from where it
// stopped the next time it is called.
//
// The invariant the whole file is built around is that nothing is ever visible
// under its real name until it is complete and verified. An interrupted run
// therefore leaves only tmp/<name>.part and runtime/.tmp-<tag>, and there is no
// reachable half-installed state for anything else to trip over.
func Install(ctx context.Context, p Progress) error {
	in, err := defaultInstaller()
	if err != nil {
		return err
	}
	return in.run(ctx, p)
}

// installer holds the resolved jobs plus the seams tests replace: the origins
// live in the artifacts, the HTTP client and the post-processing step are
// fields, and nothing else reaches outside this struct.
type installer struct {
	runtime artifact
	model   artifact
	client  *http.Client
	goos    string
	post    func(dir, bin string)

	// Retry seams. Zero means the production policy; tests shorten the wait so
	// the suite does not sleep through a real backoff.
	attempts int
	backoff  func(attempt int, hint time.Duration) time.Duration
}

func defaultInstaller() (*installer, error) {
	rt, err := runtimeArtifact(thisOS(), thisArch())
	if err != nil {
		return nil, err
	}
	return &installer{
		runtime: rt,
		model:   modelArtifact(),
		// No client Timeout: a two-gigabyte download on a slow connection is
		// legitimately long, and the caller's context is what bounds it.
		client: &http.Client{},
		goos:   thisOS(),
		post:   postProcess,
	}, nil
}

func (in *installer) run(ctx context.Context, p Progress) error {
	if err := hithome.EnsureDirs(); err != nil {
		return fmt.Errorf("llmhost: preparing %s: %w", hithome.Dir(), err)
	}
	in.gc()

	if !runtimeVerified(in.runtime.sum, in.goos) {
		if err := in.installRuntime(ctx, p); err != nil {
			return err
		}
	}
	if !modelPresentSized(in.model.bytes) {
		if err := in.installModel(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// gc clears the two kinds of leftover an interrupted run can produce. Staging
// directories always go: a partially extracted tree cannot be resumed, only
// redone. Partial downloads go only if they belong to an artifact we are no
// longer after — the current ones are exactly what resume needs.
func (in *installer) gc() {
	entries, _ := os.ReadDir(hithome.Runtime())
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), ".tmp-") {
			_ = os.RemoveAll(filepath.Join(hithome.Runtime(), e.Name()))
		}
	}

	keep := map[string]bool{
		in.runtime.file + partSuffix: true,
		in.model.file + partSuffix:   true,
	}
	parts, _ := os.ReadDir(hithome.Tmp())
	for _, e := range parts {
		if e.IsDir() || !strings.HasSuffix(e.Name(), partSuffix) || keep[e.Name()] {
			continue
		}
		_ = os.Remove(filepath.Join(hithome.Tmp(), e.Name()))
	}
}

const partSuffix = ".part"

func (in *installer) installRuntime(ctx context.Context, p Progress) error {
	part, err := in.fetch(ctx, in.runtime, p)
	if err != nil {
		return err
	}

	// Extract into a staging directory whose name marks it as garbage if we
	// die here, then swap it into place with one rename.
	stage := filepath.Join(hithome.Runtime(), ".tmp-"+RuntimeTag)
	if err := os.RemoveAll(stage); err != nil {
		return err
	}
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(stage) // no-op once the rename has succeeded

	report(p, in.runtime.label, in.runtime.bytes, in.runtime.bytes)
	if err := extractArchive(in.runtime.kind(), part, stage); err != nil {
		return fmt.Errorf("llmhost: extracting %s: %w", in.runtime.file, err)
	}

	bin, err := flatten(stage, serverBinName(in.goos))
	if err != nil {
		return fmt.Errorf("llmhost: %s: %w", in.runtime.file, err)
	}
	if in.post != nil {
		in.post(stage, bin)
	}
	if err := os.WriteFile(filepath.Join(stage, markerFile), []byte(in.runtime.sum+"\n"), 0o644); err != nil {
		return err
	}

	final := hithome.RuntimeTag(RuntimeTag)
	if err := os.RemoveAll(final); err != nil {
		return err
	}
	if err := os.Rename(stage, final); err != nil {
		return fmt.Errorf("llmhost: installing runtime: %w", err)
	}
	// "current" names the active tag and is written last, so a reader that sees
	// it can rely on the directory it names being complete.
	if err := os.WriteFile(hithome.RuntimeCurrent(), []byte(RuntimeTag+"\n"), 0o644); err != nil {
		return err
	}
	_ = os.Remove(part)
	return nil
}

func (in *installer) installModel(ctx context.Context, p Progress) error {
	part, err := in.fetch(ctx, in.model, p)
	if err != nil {
		return err
	}
	// Same filesystem (both under the home directory), so this is atomic.
	if err := os.Rename(part, ModelPath()); err != nil {
		return fmt.Errorf("llmhost: installing model: %w", err)
	}
	return nil
}

// fetch downloads an artifact into tmp/, verifies it, and returns the path of
// the verified file. A mismatch deletes the file and refuses: this is
// executable code and a model that steers it, and installing either unverified
// is not a trade-off worth making.
func (in *installer) fetch(ctx context.Context, a artifact, p Progress) (string, error) {
	part := filepath.Join(hithome.Tmp(), a.file+partSuffix)

	if err := in.download(ctx, a, part, p); err != nil {
		return "", err
	}
	if a.bytes > 0 {
		fi, err := os.Stat(part)
		if err != nil {
			return "", err
		}
		if fi.Size() != a.bytes {
			_ = os.Remove(part)
			return "", fmt.Errorf("llmhost: %s is %d bytes, expected %d", a.file, fi.Size(), a.bytes)
		}
	}

	// Hash by re-reading the completed file rather than incrementally as bytes
	// arrive: incremental hash state does not survive a resume, and re-reading
	// off an SSD costs a couple of seconds against a download measured in
	// minutes. Correctness over cleverness.
	sum, err := sha256File(part)
	if err != nil {
		return "", err
	}
	if sum != a.sum {
		_ = os.Remove(part)
		return "", fmt.Errorf("%w: %s\n  expected %s\n  got      %s", ErrChecksum, a.file, a.sum, sum)
	}
	return part, nil
}

// Retry policy for the transfer. Two gigabytes over a real network will hit a
// dropped connection, an exhausted ephemeral port or a CDN hiccup sooner or
// later, and making the user be the retry loop for that is not acceptable.
const (
	maxDownloadAttempts = 5
	baseBackoff         = time.Second
	maxBackoff          = 30 * time.Second
)

// transferError marks an attempt that failed, and whether trying again could
// plausibly help. A 404 or a bad checksum cannot be fixed by waiting; a reset
// connection or a 503 usually can.
type transferError struct {
	err        error
	retryable  bool
	retryAfter time.Duration // from Retry-After, when the server sent one
}

func (e *transferError) Error() string { return e.err.Error() }
func (e *transferError) Unwrap() error { return e.err }

// download writes the artifact to part, resuming from whatever is already
// there and retrying transient failures with bounded exponential backoff.
//
// Every attempt re-stats the .part and re-issues Range from its current size,
// so a failure costs only the bytes still outstanding rather than all of them.
// Giving up keeps the .part, so a later rerun picks up where this left off.
func (in *installer) download(ctx context.Context, a artifact, part string, p Progress) error {
	attempts := in.attempts
	if attempts <= 0 {
		attempts = maxDownloadAttempts
	}
	wait := in.backoff
	if wait == nil {
		wait = defaultBackoff
	}

	var last error
	for attempt := 1; ; attempt++ {
		err := in.downloadOnce(ctx, a, part, p)
		if err == nil {
			return nil
		}
		// The user pressing ctrl-c is not a transient failure.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var te *transferError
		if !errors.As(err, &te) || !te.retryable {
			return err
		}
		last = err
		if attempt >= attempts {
			break
		}

		// Tell the caller what is happening. A stalled-looking bar for thirty
		// seconds is indistinguishable from a hang.
		report(p, fmt.Sprintf("%s · retrying %d/%d", a.label, attempt+1, attempts), partSize(part), a.bytes)

		select {
		case <-time.After(wait(attempt, te.retryAfter)):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("llmhost: downloading %s: gave up after %d attempts (%d of %d bytes fetched, rerun to resume): %w",
		a.file, attempts, partSize(part), a.bytes, last)
}

// defaultBackoff doubles each attempt up to a ceiling, with jitter over the top
// half of the interval so concurrent clients do not synchronise into a retry
// storm against the same origin. A server's own Retry-After always wins.
func defaultBackoff(attempt int, hint time.Duration) time.Duration {
	if hint > 0 {
		return min(hint, maxBackoff)
	}
	d := min(baseBackoff<<(attempt-1), maxBackoff)
	return d/2 + time.Duration(rand.Float64()*float64(d/2))
}

func partSize(part string) int64 {
	fi, err := os.Stat(part)
	if err != nil {
		return 0
	}
	return fi.Size()
}

// retryAfter reads the header of the same name, which is either a count of
// seconds or an HTTP date.
func retryAfter(h http.Header) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if when, err := http.ParseTime(v); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return 0
}

// downloadOnce is a single transfer attempt.
func (in *installer) downloadOnce(ctx context.Context, a artifact, part string, p Progress) error {
	var have int64
	if fi, err := os.Stat(part); err == nil && !fi.IsDir() {
		have = fi.Size()
		switch {
		case a.bytes > 0 && have == a.bytes:
			// Already complete from a previous run; the caller hashes it.
			report(p, a.label, have, a.bytes)
			return nil
		case a.bytes > 0 && have > a.bytes:
			have = 0 // longer than the artifact: it is not a prefix of it
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.url, nil)
	if err != nil {
		return err
	}
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	resp, err := in.client.Do(req)
	if err != nil {
		// Anything that failed at the transport — refused, reset, no ephemeral
		// port, DNS blip — is exactly what retrying is for.
		return &transferError{
			err:       fmt.Errorf("llmhost: downloading %s: %w", a.file, err),
			retryable: true,
		}
	}
	defer resp.Body.Close()

	flag := os.O_CREATE | os.O_WRONLY
	switch {
	case resp.StatusCode == http.StatusPartialContent:
		flag |= os.O_APPEND
	case resp.StatusCode == http.StatusOK:
		// The server ignored Range and is sending the whole thing from byte
		// zero. Appending that to a partial file would produce a corrupt
		// artifact that only the hash would catch, so start over.
		have = 0
		flag |= os.O_TRUNC
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		// The .part is at or past the end of the resource, so it is not a
		// prefix of it. Discard it and let the next attempt start clean.
		_ = os.Remove(part)
		return &transferError{
			err:       fmt.Errorf("llmhost: downloading %s: %s", a.file, resp.Status),
			retryable: true,
		}
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return &transferError{
			err:        fmt.Errorf("llmhost: downloading %s: %s", a.file, resp.Status),
			retryable:  true,
			retryAfter: retryAfter(resp.Header),
		}
	default:
		// Any other 4xx is about the request, not the network. Retrying it
		// would just be five identical failures and a minute of the user's
		// time.
		return fmt.Errorf("llmhost: downloading %s: %s", a.file, resp.Status)
	}

	total := a.bytes
	if total <= 0 && resp.ContentLength > 0 {
		total = have + resp.ContentLength
	}

	f, err := os.OpenFile(part, flag, 0o644)
	if err != nil {
		return err
	}
	pw := &progressWriter{w: f, label: a.label, done: have, total: total, report: p}
	_, copyErr := io.Copy(pw, resp.Body)
	pw.flush()
	syncErr := f.Sync()
	closeErr := f.Close()

	switch {
	case copyErr != nil:
		if ctx.Err() != nil {
			return ctx.Err() // interrupted: the .part stays and resumes
		}
		// A body that stopped early — reset connection, unexpected EOF — keeps
		// whatever did arrive, so the next attempt resumes from there rather
		// than starting over.
		return &transferError{
			err:       fmt.Errorf("llmhost: downloading %s: %w", a.file, copyErr),
			retryable: true,
		}
	case syncErr != nil:
		return syncErr
	default:
		return closeErr
	}
}

func report(p Progress, label string, done, total int64) {
	if p != nil {
		p(label, done, total)
	}
}

// progressWriter throttles callbacks. A 2GB download at 20MB/s is ~130,000
// 16KB writes; the caller is drawing a terminal bar and does not want them all.
type progressWriter struct {
	w      io.Writer
	label  string
	done   int64
	total  int64
	report Progress
	last   time.Time
	sent   bool
}

const progressInterval = 100 * time.Millisecond

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	if !p.sent || time.Since(p.last) >= progressInterval {
		p.last, p.sent = time.Now(), true
		report(p.report, p.label, p.done, p.total)
	}
	return n, err
}

// flush emits the final position, so a bar always lands on its total rather
// than stopping at whatever the last throttled tick happened to be.
func (p *progressWriter) flush() { report(p.report, p.label, p.done, p.total) }

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractArchive unpacks src into dest. Both formats are needed: macOS and
// Linux ship .tar.gz and only Windows ships .zip.
func extractArchive(kind, src, dest string) error {
	if kind == "zip" {
		return extractZip(src, dest)
	}
	return extractTarGz(src, dest)
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safePath(dest, h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeEntry(target, tr, fs.FileMode(h.Mode).Perm()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// A symlink pointing out of the tree is the same escape as a "../"
			// path, just deferred. Skipped rather than fatal: these archives
			// use links for library versioning, and a dropped link is at worst
			// a missing alias the binary does not use.
			if !linkStaysInside(h.Name, h.Linkname) {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(h.Linkname, target); err != nil {
				return err
			}
		default:
			// Devices, fifos and hard links have no business in a release
			// archive of binaries.
			continue
		}
	}
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target, err := safePath(dest, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeEntry(target, rc, f.Mode().Perm())
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeEntry(target string, src io.Reader, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	// The archive was hash-verified before it got here, so its contents are
	// exactly the bytes we pinned — there is no decompression bomb to defend
	// against that the hash has not already ruled out.
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// safePath resolves an archive entry against dest and refuses anything that
// would land outside it. filepath.IsLocal is the whole check: it rejects
// absolute paths, any path that climbs out with "..", and on Windows the
// reserved device names, which is more than a prefix comparison catches.
func safePath(dest, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	clean = strings.TrimSuffix(clean, string(os.PathSeparator))
	if clean == "" || clean == "." {
		return "", fmt.Errorf("%w: %q", ErrUnsafeArchive, name)
	}
	if !filepath.IsLocal(clean) {
		return "", fmt.Errorf("%w: %q", ErrUnsafeArchive, name)
	}
	return filepath.Join(dest, clean), nil
}

// linkStaysInside reports whether a symlink's target, resolved relative to the
// link's own directory, remains inside the extraction root.
func linkStaysInside(entry, link string) bool {
	if link == "" || path.IsAbs(link) || filepath.IsAbs(link) {
		return false
	}
	resolved := path.Join(path.Dir(filepath.ToSlash(entry)), filepath.ToSlash(link))
	return filepath.IsLocal(filepath.FromSlash(resolved))
}

// flatten finds the server binary anywhere under dir and, when it is nested,
// lifts its directory's contents to the top.
//
// Locating rather than assuming is deliberate: the macOS and Linux tarballs
// nest everything under build/bin while the Windows zips put it at the root,
// and a hard-coded path would be a guess that fails at install time on a
// platform nobody tested. Finding the binary works for either layout, and for
// whatever the next release does.
func flatten(dir, binName string) (string, error) {
	found, err := locate(dir, binName)
	if err != nil {
		return "", err
	}
	src := filepath.Dir(found)
	if src == dir {
		return found, nil
	}

	rel, err := filepath.Rel(dir, src)
	if err != nil {
		return "", err
	}
	top := strings.Split(rel, string(os.PathSeparator))[0]

	entries, err := os.ReadDir(src)
	if err != nil {
		return "", err
	}
	collides := false
	for _, e := range entries {
		if e.Name() == top {
			collides = true
		}
		if err := os.Rename(filepath.Join(src, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			return "", err
		}
	}
	if !collides {
		_ = os.RemoveAll(filepath.Join(dir, top))
	}
	return filepath.Join(dir, binName), nil
}

func locate(dir, name string) (string, error) {
	var found string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Name() == name {
			found = p
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("no %s in the archive", name)
	}
	return found, nil
}
