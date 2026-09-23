// Package selfupdate replaces this executable with a newer release build.
//
// It resolves the latest tag from the repository's releases, downloads the
// asset for this GOOS/GOARCH, verifies its SHA-256 against the published
// checksums, and swaps it into place — the same sequence install.sh performs,
// so a binary installed either way ends up identical.
//
// Non-goals: it does not run migrations, does not touch ~/.hittable, does not
// phone home, and never downgrades silently. It refuses to install anything
// whose checksum it cannot verify, because the artifact it is fetching is code
// that will then be executed.
package selfupdate

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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// executable is os.Executable, indirected so the tests can point Apply at a
// throwaway file instead of the test binary itself — which it would otherwise
// overwrite.
var executable = os.Executable

// ErrUpToDate means the running version is already the latest release.
var ErrUpToDate = errors.New("selfupdate: already on the latest version")

const (
	// DefaultRepo is the source of releases.
	DefaultRepo = "Pantho-Haque/hittable"
	binName     = "hittable"
	// maxAsset bounds a download so a wrong URL cannot fill the disk.
	maxAsset = 200 << 20
)

// Config describes where to look and what is running. The URL fields exist so
// the tests can point at an httptest server; leave them empty in production.
type Config struct {
	Repo    string
	Current string // the running version, e.g. "shellapp-v1.2.0" or "dev"
	Client  *http.Client
	Site    string // default https://github.com
	API     string // default https://api.github.com
	GOOS    string // defaults to runtime.GOOS
	GOARCH  string // defaults to runtime.GOARCH
}

func (c *Config) fill() {
	if c.Repo == "" {
		c.Repo = DefaultRepo
	}
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 5 * time.Minute}
	}
	if c.Site == "" {
		c.Site = "https://github.com"
	}
	if c.API == "" {
		c.API = "https://api.github.com"
	}
	if c.GOOS == "" {
		c.GOOS = runtime.GOOS
	}
	if c.GOARCH == "" {
		c.GOARCH = runtime.GOARCH
	}
}

// Progress reports download bytes. total is 0 when the server sends no length.
type Progress func(done, total int64)

// Latest returns the most recent release tag.
func Latest(ctx context.Context, cfg Config) (string, error) {
	cfg.fill()
	url := cfg.API + "/repos/" + cfg.Repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := cfg.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("selfupdate: checking for a newer version: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("selfupdate: the release API answered %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	tag := jsonString(string(body), "tag_name")
	if tag == "" {
		return "", errors.New("selfupdate: no tag_name in the release response")
	}
	return tag, nil
}

// jsonString pulls one top-level string field out without a struct, so this
// package stays free of a JSON shape it does not otherwise care about.
func jsonString(body, key string) string {
	needle := `"` + key + `"`
	i := strings.Index(body, needle)
	if i < 0 {
		return ""
	}
	rest := body[i+len(needle):]
	if j := strings.Index(rest, `"`); j >= 0 {
		rest = rest[j+1:]
		if k := strings.Index(rest, `"`); k >= 0 {
			return rest[:k]
		}
	}
	return ""
}

// AssetName is the published file name for a tag on this platform.
func AssetName(tag, goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("%s_%s_%s_%s.%s", binName, tag, goos, goarch, ext)
}

// Apply downloads tag and replaces the running executable with it. It returns
// the path that was written.
func Apply(ctx context.Context, cfg Config, tag string, p Progress) (string, error) {
	cfg.fill()

	target, err := executable()
	if err != nil {
		return "", fmt.Errorf("selfupdate: locating this executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		target = resolved
	}
	// Fail here rather than after a 25MB download.
	if err := writable(filepath.Dir(target)); err != nil {
		return "", fmt.Errorf("selfupdate: %s is not writable (%w)\n"+
			"       reinstall with: curl -fsSL https://raw.githubusercontent.com/%s/main/install.sh | sh",
			filepath.Dir(target), err, cfg.Repo)
	}

	asset := AssetName(tag, cfg.GOOS, cfg.GOARCH)
	base := cfg.Site + "/" + cfg.Repo + "/releases/download/" + tag

	dir, err := os.MkdirTemp("", "hittable-update-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	archive := filepath.Join(dir, asset)
	if err := download(ctx, cfg, base+"/"+asset, archive, p); err != nil {
		return "", fmt.Errorf("selfupdate: no build for %s/%s in %s (%w)", cfg.GOOS, cfg.GOARCH, tag, err)
	}

	// Verified before anything is unpacked: an artifact that fails here is code
	// we were about to run.
	sums := filepath.Join(dir, "checksums.txt")
	if err := download(ctx, cfg, base+"/checksums.txt", sums, nil); err != nil {
		return "", fmt.Errorf("selfupdate: %s publishes no checksums; refusing to install unverified code", tag)
	}
	want, err := sumFor(sums, asset)
	if err != nil {
		return "", fmt.Errorf("selfupdate: %w", err)
	}
	got, err := sha256File(archive)
	if err != nil {
		return "", err
	}
	if want != got {
		return "", fmt.Errorf("selfupdate: checksum mismatch for %s\n       expected %s\n       actual   %s", asset, want, got)
	}

	fresh := filepath.Join(dir, binName+exeSuffix(cfg.GOOS))
	if err := extract(archive, dir, cfg.GOOS); err != nil {
		return "", fmt.Errorf("selfupdate: unpacking: %w", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		return "", fmt.Errorf("selfupdate: %s did not contain %s", asset, filepath.Base(fresh))
	}
	if err := os.Chmod(fresh, 0o755); err != nil {
		return "", err
	}
	sign(fresh, cfg.GOOS)

	if err := swap(fresh, target, cfg.GOOS); err != nil {
		return "", fmt.Errorf("selfupdate: replacing %s: %w", target, err)
	}
	return target, nil
}

func exeSuffix(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}

// sign gives the binary an ad-hoc signature. An arm64 Mach-O with no signature
// is killed by the kernel on Apple silicon, and release builds are
// cross-compiled and therefore unsigned. Best effort: a machine without the
// developer tools still gets a working install on Intel.
func sign(path, goos string) {
	if goos != "darwin" || runtime.GOOS != "darwin" {
		return
	}
	_ = exec.Command("xattr", "-d", "com.apple.quarantine", path).Run()
	_ = exec.Command("codesign", "--force", "--sign", "-", path).Run()
}

// swap puts fresh at target. On Unix a rename over a running executable is
// allowed — it unlinks the old inode, which the running process keeps open —
// and is atomic, so an interrupted update cannot leave a half-written binary.
// Windows refuses to replace a file that is executing, so the old one is moved
// aside first and cleaned up on the next run.
func swap(fresh, target, goos string) error {
	if goos == "windows" {
		old := target + ".old"
		_ = os.Remove(old)
		if err := os.Rename(target, old); err != nil {
			return err
		}
		if err := os.Rename(fresh, target); err != nil {
			_ = os.Rename(old, target) // put it back
			return err
		}
		return nil
	}
	// Same filesystem is required for rename; the temp dir may be elsewhere, so
	// stage beside the target first.
	staged := filepath.Join(filepath.Dir(target), "."+binName+".new")
	if err := copyFile(fresh, staged); err != nil {
		return err
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		_ = os.Remove(staged)
		return err
	}
	if err := os.Rename(staged, target); err != nil {
		_ = os.Remove(staged)
		return err
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func writable(dir string) error {
	f, err := os.CreateTemp(dir, ".hittable-write-check-")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}

func download(ctx context.Context, cfg Config, url, dst string, p Progress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := cfg.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	var w io.Writer = f
	if p != nil {
		w = &progressWriter{w: f, total: resp.ContentLength, report: p}
	}
	_, err = io.Copy(w, io.LimitReader(resp.Body, maxAsset))
	return err
}

type progressWriter struct {
	w      io.Writer
	done   int64
	total  int64
	report Progress
	last   time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	// Throttled: a redraw per 32KB chunk is thousands of writes to a terminal.
	if time.Since(p.last) > 100*time.Millisecond || err != nil {
		p.last = time.Now()
		p.report(p.done, p.total)
	}
	return n, err
}

func sumFor(checksums, asset string) (string, error) {
	b, err := os.ReadFile(checksums)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == asset {
			return f[0], nil
		}
	}
	return "", fmt.Errorf("%s is not listed in checksums.txt", asset)
}

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

func extract(archive, dir, goos string) error {
	if goos == "windows" {
		return extractZip(archive, dir)
	}
	return extractTarGz(archive, dir)
}

// safeJoin refuses an entry whose path escapes dir. An archive is untrusted
// input even when we published it.
func safeJoin(dir, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || !filepath.IsLocal(clean) {
		return "", fmt.Errorf("refusing archive entry %q", name)
	}
	return filepath.Join(dir, clean), nil
}

func extractTarGz(archive, dir string) error {
	f, err := os.Open(archive)
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
		if h.Typeflag != tar.TypeReg {
			continue
		}
		path, err := safeJoin(dir, h.Name)
		if err != nil {
			return err
		}
		if err := writeEntry(path, tr); err != nil {
			return err
		}
	}
}

func extractZip(archive, dir string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, e := range zr.File {
		if e.FileInfo().IsDir() {
			continue
		}
		path, err := safeJoin(dir, e.Name)
		if err != nil {
			return err
		}
		rc, err := e.Open()
		if err != nil {
			return err
		}
		err = writeEntry(path, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeEntry(path string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, io.LimitReader(r, maxAsset)); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
