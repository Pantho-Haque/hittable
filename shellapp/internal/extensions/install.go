package extensions

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hittable/shellapp/internal/hithome"
	"github.com/hittable/shellapp/internal/llmhost"
)

// Progress reports download progress, as llmhost.Progress does.
type Progress = llmhost.Progress

// Result describes a finished install.
type Result struct {
	Installed Installed
	Deps      []string // ids installed along with it
	Warnings  []string
}

// blocked are Microsoft extensions whose licences allow them only in
// Microsoft's VS Code. They are refused even from a local .vsix.
var blocked = map[string]bool{
	"ms-vscode.cpptools":                     true,
	"ms-python.vscode-pylance":               true,
	"ms-vscode-remote.remote-ssh":            true,
	"ms-vscode-remote.remote-containers":     true,
	"ms-vscode-remote.remote-wsl":            true,
	"ms-vscode.remote-server":                true,
	"ms-vscode.remote-explorer":              true,
	"ms-vsliveshare.vsliveshare":             true,
	"github.copilot":                         true,
	"github.copilot-chat":                    true,
	"ms-dotnettools.csdevkit":                true,
	"visualstudioexptteam.vscodeintellicode": true,
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.[a-z0-9][a-z0-9._-]*$`)

// ValidID reports whether s looks like publisher.name.
func ValidID(s string) bool { return idPattern.MatchString(strings.ToLower(s)) }

// Install installs spec: "publisher.name", "publisher.name@1.2.3",
// "publisher.name@latest" (update an installed one), or the
// path of a .vsix file. Dependencies (extensionDependencies and
// extensionPack) are installed too. Installing an id that is already at the
// chosen version is a no-op that still re-enables it.
func Install(ctx context.Context, spec string, p Progress) (Result, error) {
	if err := hithome.EnsureDirs(); err != nil {
		return Result{}, err
	}
	unlock, err := lock()
	if err != nil {
		return Result{}, err
	}
	defer unlock()
	gcStaging()
	return install(ctx, spec, p, map[string]bool{})
}

func install(ctx context.Context, spec string, p Progress, seen map[string]bool) (Result, error) {
	var res Result
	isFile := strings.HasSuffix(strings.ToLower(spec), ".vsix")

	var rel Release
	var vsix string
	source := "openvsx"
	if isFile {
		abs, err := filepath.Abs(expandHome(spec))
		if err != nil {
			return res, err
		}
		if _, err := os.Stat(abs); err != nil {
			return res, fmt.Errorf("no such file: %s", spec)
		}
		vsix, source = abs, "vsix"
	} else {
		id, version, _ := strings.Cut(strings.ToLower(strings.TrimSpace(spec)), "@")
		// "@latest" re-resolves even when installed: that is an update.
		force := version == "latest"
		if force {
			version = ""
		}
		if !ValidID(id) {
			return res, fmt.Errorf("%q is not an extension id (publisher.name) or a .vsix file", spec)
		}
		if blocked[id] {
			return res, blockedErr(id)
		}
		if seen[id] {
			return res, nil // a dependency cycle, or a pack listing itself
		}
		seen[id] = true
		if cur, ok := findIn(id); ok && version == "" && !force {
			// Already here: only re-resolve when asked for a version.
			if !cur.Enabled {
				_ = setEnabledLocked(id, true)
				cur.Enabled = true
			}
			res.Installed = cur
			return res, nil
		}
		var err error
		rel, err = Resolve(ctx, id, version)
		if err != nil {
			return res, err
		}
		vsix, err = downloadRelease(ctx, rel, p)
		if err != nil {
			return res, err
		}
		defer os.Remove(vsix)
	}

	signed := false
	if rel.Signature != "" && rel.PublicKey != "" {
		if err := verifySignature(ctx, vsix, rel); err != nil {
			return res, err
		}
		signed = true
	} else if source == "openvsx" {
		res.Warnings = append(res.Warnings, rel.ID+" is not signed by the registry")
	}

	stage, err := os.MkdirTemp(hithome.Extensions(), ".tmp-")
	if err != nil {
		return res, err
	}
	defer os.RemoveAll(stage)
	if err := unpackVSIX(vsix, stage); err != nil {
		return res, err
	}
	root := filepath.Join(stage, "extension")
	m, _, err := ReadManifest(root)
	if err != nil {
		return res, fmt.Errorf("not a valid extension: %w", err)
	}
	id := m.ID()
	if blocked[id] {
		return res, blockedErr(id)
	}
	if rel.ID != "" && id != rel.ID {
		return res, fmt.Errorf("the registry served %s for %s; refusing it", id, rel.ID)
	}
	if len(m.APIProposals) > 0 {
		return res, fmt.Errorf("%s uses proposed VS Code APIs (%s), which only VS Code Insiders allows",
			id, strings.Join(m.APIProposals, ", "))
	}
	if eng := m.Engines["vscode"]; !EngineAllows(eng, APIVersion) {
		return res, fmt.Errorf("%s %s needs VS Code %s; hittable provides the %s API", id, m.Version, eng, APIVersion)
	}
	if u := m.Unsupported(); len(u) > 0 {
		res.Warnings = append(res.Warnings, id+" also contributes "+strings.Join(u, ", ")+", which the terminal app cannot show")
	}
	seen[id] = true

	// Dependencies first: an extension activated before what it requires
	// fails in extensions.getExtension(...).activate().
	for _, dep := range m.Dependencies {
		r, err := install(ctx, dep, p, seen)
		if err != nil {
			return res, fmt.Errorf("%s needs %s: %w", id, dep, err)
		}
		if r.Installed.ID != "" {
			res.Deps = append(res.Deps, r.Installed.ID)
		}
		res.Warnings = append(res.Warnings, r.Warnings...)
	}
	for _, dep := range m.Pack {
		r, err := install(ctx, dep, p, seen)
		if err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("pack member %s skipped: %v", dep, err))
			continue
		}
		if r.Installed.ID != "" {
			res.Deps = append(res.Deps, r.Installed.ID)
		}
	}

	dirName := id + "-" + m.Version
	if rel.TargetPlatform != "" && rel.TargetPlatform != "universal" {
		dirName += "-" + rel.TargetPlatform
	}
	final := filepath.Join(hithome.Extensions(), dirName)
	_ = os.RemoveAll(final)
	if err := os.Rename(root, final); err != nil {
		return res, err
	}

	ix, err := readIndex()
	if err != nil {
		return res, err
	}
	entry := Installed{ID: id, Version: m.Version, Dir: dirName, Enabled: true, Source: source,
		TargetPlatform: rel.TargetPlatform, Signed: signed, InstalledAt: time.Now().UTC()}
	var old []string
	kept := ix.Extensions[:0]
	for _, e := range ix.Extensions {
		if e.ID == id {
			if e.Dir != dirName {
				old = append(old, e.Path())
			}
			continue
		}
		kept = append(kept, e)
	}
	ix.Extensions = append(kept, entry)
	if err := writeIndex(ix); err != nil {
		return res, err
	}
	for _, o := range old {
		_ = os.RemoveAll(o) // a running host keeps its open files on Unix
	}
	entry.Manifest = m
	res.Installed = entry
	return res, nil
}

func blockedErr(id string) error {
	return fmt.Errorf("%s is licensed by Microsoft for use only in Microsoft's VS Code, so hittable cannot run it", id)
}

func findIn(id string) (Installed, bool) { return Find(id) }

func setEnabledLocked(id string, on bool) error {
	ix, err := readIndex()
	if err != nil {
		return err
	}
	for i := range ix.Extensions {
		if ix.Extensions[i].ID == id {
			ix.Extensions[i].Enabled = on
		}
	}
	return writeIndex(ix)
}

// downloadRelease fetches the .vsix, checked against the registry's SHA-256.
func downloadRelease(ctx context.Context, rel Release, p Progress) (string, error) {
	sum := ""
	if rel.SHA256 != "" {
		b, err := fetchSmall(ctx, rel.SHA256, 1024)
		if err != nil {
			return "", fmt.Errorf("reading the checksum for %s: %w", rel.ID, err)
		}
		if f := strings.Fields(string(b)); len(f) > 0 {
			sum = strings.ToLower(f[0])
		}
	}
	file := rel.ID + "-" + rel.Version + ".vsix"
	label := rel.ID + " " + rel.Version
	if sum == "" {
		return "", fmt.Errorf("the registry published no checksum for %s %s; refusing an unverifiable download", rel.ID, rel.Version)
	}
	return llmhost.DownloadVerified(ctx, label, rel.Download, file, 0, sum, p)
}

// verifySignature checks the registry's Ed25519 signature over the raw
// .vsix bytes. The .sigzip holds .signature.sig (64 raw bytes); the key is
// the PEM the registry names for this publisher.
func verifySignature(ctx context.Context, vsix string, rel Release) error {
	sigzip, err := fetchSmall(ctx, rel.Signature, 4<<20)
	if err != nil {
		return fmt.Errorf("reading the signature of %s: %w", rel.ID, err)
	}
	sig, err := zipMember(sigzip, ".signature.sig")
	if err != nil {
		return fmt.Errorf("signature of %s: %w", rel.ID, err)
	}
	keyPEM, err := fetchSmall(ctx, rel.PublicKey, 16<<10)
	if err != nil {
		return fmt.Errorf("reading the signing key of %s: %w", rel.ID, err)
	}
	blk, _ := pem.Decode(keyPEM)
	if blk == nil {
		return fmt.Errorf("signing key of %s is not PEM", rel.ID)
	}
	k, err := x509.ParsePKIXPublicKey(blk.Bytes)
	if err != nil {
		return fmt.Errorf("signing key of %s: %w", rel.ID, err)
	}
	pub, ok := k.(ed25519.PublicKey)
	if !ok {
		return fmt.Errorf("signing key of %s is not Ed25519", rel.ID)
	}
	data, err := os.ReadFile(vsix)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, data, sig) {
		return fmt.Errorf("%s %s failed signature verification; not installed", rel.ID, rel.Version)
	}
	return nil
}

func zipMember(b []byte, name string) ([]byte, error) {
	r, err := zip.NewReader(bytesReaderAt(b), int64(len(b)))
	if err != nil {
		return nil, err
	}
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, 1<<20))
		}
	}
	return nil, fmt.Errorf("no %s", name)
}

type bytesReaderAt []byte

func (b bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, io.EOF
	}
	n := copy(p, b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// unpackVSIX extracts a .vsix (a zip whose extension lives under
// extension/) into dest, keeping Unix file modes so bundled language-server
// binaries stay executable. Entries that would escape dest are refused.
func unpackVSIX(vsix, dest string) error {
	if err := llmhost.ExtractArchive("zip", vsix, dest); err != nil {
		if errors.Is(err, zip.ErrFormat) {
			return fmt.Errorf("%s is not a .vsix (zip) file", filepath.Base(vsix))
		}
		return fmt.Errorf("unpacking %s: %w", filepath.Base(vsix), err)
	}
	if _, err := os.Stat(filepath.Join(dest, "extension", "package.json")); err != nil {
		return fmt.Errorf("%s has no extension/package.json", filepath.Base(vsix))
	}
	fixModes(filepath.Join(dest, "extension"))
	return nil
}

// fixModes marks files under bin/ or server/ folders executable when the
// zip carried no Unix mode at all (vsce on Windows writes none), the usual
// cause of "permission denied" from a bundled server.
func fixModes(root string) {
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		inBin := false
		for _, s := range parts[:len(parts)-1] {
			if s == "bin" || s == "server" {
				inBin = true
			}
		}
		if !inBin || strings.Contains(d.Name(), ".") && !strings.HasSuffix(d.Name(), ".sh") {
			return nil
		}
		if fi, err := d.Info(); err == nil && fi.Mode().Perm()&0o111 == 0 {
			_ = os.Chmod(p, fi.Mode().Perm()|0o755)
		}
		return nil
	})
}

// gcStaging removes half-unpacked folders left by an interrupted install.
func gcStaging() {
	entries, _ := os.ReadDir(hithome.Extensions())
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), ".tmp-") {
			_ = os.RemoveAll(filepath.Join(hithome.Extensions(), e.Name()))
		}
	}
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}
