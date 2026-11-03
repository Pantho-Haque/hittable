// Package noderuntime finds the Node.js that runs VS Code extensions, and
// installs one globally when the machine has none.
//
// The Node already on the machine is used: HITTABLE_NODE when set, then
// `node` on PATH, then the usual install locations (Homebrew, nvm, fnm,
// Volta, asdf, mise, the official installers), newest acceptable first.
// When none is new enough, InstallGlobal installs Node for the whole
// account rather than privately for hittable: with Homebrew when it is
// there (macOS, Linuxbrew), with winget on Windows, otherwise the official
// build unpacked to ~/.local/lib/nodejs with node, npm and npx linked into
// ~/.local/bin. Nothing here asks for sudo.
package noderuntime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hittable/shellapp/internal/hithome"
	"github.com/hittable/shellapp/internal/llmhost"
)

// Version is the Node.js LTS installed when the machine has none.
const Version = "v24.22.2"

// MinMajor is the oldest Node extensions are run with. vscode-languageclient,
// which most language extensions bundle, needs 18+; 20 is the oldest line
// still getting security fixes.
const MinMajor = 20

// EnvOverride names the variable that points at a Node binary to use.
const EnvOverride = "HITTABLE_NODE"

const origin = "https://nodejs.org/dist/" + Version + "/"

type asset struct {
	file  string
	bytes int64
	sum   string
}

// assets were read from nodejs.org/dist/<Version>/SHASUMS256.txt and the
// files' Content-Length, not estimated.
var assets = map[string]asset{
	"darwin/arm64":  {"node-v24.22.2-darwin-arm64.tar.gz", 53068959, "aa9549349e47dffaa42dcf95d6e06ade8c4bdafc980979883b88767670df385a"},
	"darwin/amd64":  {"node-v24.22.2-darwin-x64.tar.gz", 54383415, "d5e336d588c45be0c37dbd52cc29895593009ca4997a77a1fd02815131ed0e52"},
	"linux/amd64":   {"node-v24.22.2-linux-x64.tar.gz", 58299872, "361718c444e49af2ca5a110c83ba6a8901143f23ab285ebbef0341f391f482a9"},
	"linux/arm64":   {"node-v24.22.2-linux-arm64.tar.gz", 58030966, "95c264150aceb7f3437a29fd6984924f5f8d1511ea3cbd6c508b83f6c555dae2"},
	"windows/amd64": {"node-v24.22.2-win-x64.zip", 37754834, "ca1bff5d8ead13c9f2ea404a176db46bd430704d5b5113734049fa365e7799e2"},
	"windows/arm64": {"node-v24.22.2-win-arm64.zip", 33789289, "f47968c213105077f64c6510f007632c0ca2574802a0b5067c327c49abe1c468"},
}

// DownloadBytes is the size of the official build for this machine.
func DownloadBytes() int64 { return assets[platform()].bytes }

var (
	// ErrNotInstalled: no Node anywhere on the machine.
	ErrNotInstalled = errors.New("Node.js is not installed")
	// ErrTooOld: Node is installed, but older than MinMajor.
	ErrTooOld = errors.New("the installed Node.js is too old for extensions")
	// ErrUnsupported: no official build runs here and no package manager is known.
	ErrUnsupported = errors.New("no official Node.js build runs on this system")
)

// Found is a usable Node.
type Found struct {
	Path    string
	Version string // "v22.11.0"
	Source  string // where it came from: "PATH", "Homebrew", "nvm", HITTABLE_NODE…
}

// searchSystem includes the fixed system locations (/opt/homebrew, /usr/…)
// and the brew lookup; tests turn it off to see only what they set up.
var searchSystem = true

var (
	findMu   sync.Mutex
	findAt   time.Time
	findRes  Found
	findErr  error
	versions = map[string]string{} // path|mtime -> version, so --version runs once per binary
)

// Find locates the Node to run extensions with. The answer is cached for a
// few seconds: the UI asks every frame. Forget clears it after an install.
func Find() (Found, error) {
	findMu.Lock()
	defer findMu.Unlock()
	if !findAt.IsZero() && time.Since(findAt) < 5*time.Second {
		return findRes, findErr
	}
	findRes, findErr = find()
	findAt = time.Now()
	return findRes, findErr
}

// Forget drops the cached Find result.
func Forget() {
	findMu.Lock()
	findAt = time.Time{}
	findMu.Unlock()
}

// Path is Find's binary.
func Path() (string, error) {
	f, err := Find()
	return f.Path, err
}

// Installed reports whether a usable Node exists.
func Installed() bool { _, err := Find(); return err == nil }

func find() (Found, error) {
	if p := strings.TrimSpace(os.Getenv(EnvOverride)); p != "" {
		v, err := CheckBinary(p)
		if err != nil {
			return Found{}, fmt.Errorf("%s=%s: %w", EnvOverride, p, err)
		}
		return Found{Path: p, Version: v, Source: EnvOverride}, nil
	}
	var old Found
	for _, c := range candidates() {
		v, err := versionOf(c.Path)
		if err != nil {
			continue
		}
		if major(v) < MinMajor {
			if old.Path == "" {
				old = Found{Path: c.Path, Version: v, Source: c.Source}
			}
			continue
		}
		c.Version = v
		return c, nil
	}
	if old.Path != "" {
		return old, fmt.Errorf("%w: %s at %s is Node %s; extensions need %d or newer", ErrTooOld, old.Source, old.Path, old.Version, MinMajor)
	}
	return Found{}, ErrNotInstalled
}

// candidates lists Node binaries in the order they are preferred: what the
// shell would run, then version managers (newest version first), then the
// standard install locations, then what an older hittable downloaded.
func candidates() []Found {
	var out []Found
	seen := map[string]bool{}
	add := func(p, src string) {
		if p == "" {
			return
		}
		if r, err := filepath.EvalSymlinks(p); err == nil {
			if seen[r] {
				return
			}
			seen[r] = true
		}
		if isFile(p) {
			out = append(out, Found{Path: p, Source: src})
		}
	}
	exe := "node"
	if runtime.GOOS == "windows" {
		exe = "node.exe"
	}
	if p, err := exec.LookPath("node"); err == nil {
		add(p, "PATH")
	}
	home, _ := os.UserHomeDir()
	newest := func(glob, src string) {
		ms, _ := filepath.Glob(glob)
		sort.Slice(ms, func(i, j int) bool { return versionKey(ms[i]) > versionKey(ms[j]) })
		for _, m := range ms {
			add(m, src)
		}
	}
	if home != "" {
		newest(filepath.Join(home, ".nvm", "versions", "node", "*", "bin", exe), "nvm")
		newest(filepath.Join(home, ".local", "share", "fnm", "node-versions", "*", "installation", "bin", exe), "fnm")
		newest(filepath.Join(home, "Library", "Application Support", "fnm", "node-versions", "*", "installation", "bin", exe), "fnm")
		add(filepath.Join(home, ".volta", "bin", exe), "Volta")
		add(filepath.Join(home, ".asdf", "shims", exe), "asdf")
		add(filepath.Join(home, ".local", "share", "mise", "shims", exe), "mise")
		add(filepath.Join(home, ".local", "bin", exe), "~/.local/bin")
	}
	if !searchSystem {
		return out
	}
	switch runtime.GOOS {
	case "darwin":
		add("/opt/homebrew/bin/node", "Homebrew")
		add("/usr/local/bin/node", "/usr/local/bin")
	case "linux":
		add("/home/linuxbrew/.linuxbrew/bin/node", "Homebrew")
		add("/usr/local/bin/node", "/usr/local/bin")
		add("/usr/bin/node", "/usr/bin")
		add("/snap/bin/node", "snap")
	case "windows":
		add(filepath.Join(os.Getenv("ProgramFiles"), "nodejs", exe), "Program Files")
		add(filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "nodejs", exe), "LocalAppData")
	}
	// What this version and older versions of hittable installed.
	newest(filepath.Join(localNodeRoot(), "node-v*", "bin", exe), "installed by hittable")
	newest(filepath.Join(localNodeRoot(), "node-v*", exe), "installed by hittable")
	newest(filepath.Join(hithome.Node(), "v*", "bin", exe), "an older hittable")
	return out
}

func versionKey(p string) string {
	// Zero-pad each part so v9 sorts below v10.
	var parts []string
	for _, seg := range strings.Split(p, string(filepath.Separator)) {
		v := strings.TrimPrefix(strings.TrimPrefix(seg, "node-"), "v")
		if n := strings.SplitN(v, ".", 3); len(n) == 3 {
			for _, x := range n {
				x = strings.SplitN(x, "-", 2)[0]
				parts = append(parts, fmt.Sprintf("%06s", x))
			}
		}
	}
	return strings.Join(parts, ".")
}

func versionOf(p string) (string, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	key := p + "|" + fi.ModTime().String()
	if v, ok := versions[key]; ok {
		return v, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, p, "--version").Output()
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(out))
	if !strings.HasPrefix(v, "v") {
		return "", fmt.Errorf("unexpected version %q", v)
	}
	versions[key] = v
	return v, nil
}

func major(v string) int {
	n, _ := strconv.Atoi(strings.SplitN(strings.TrimPrefix(v, "v"), ".", 2)[0])
	return n
}

// CheckBinary runs `node --version` and enforces MinMajor.
func CheckBinary(p string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, p, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("cannot run it: %w", err)
	}
	v := strings.TrimSpace(string(out))
	if major(v) < MinMajor {
		return v, fmt.Errorf("it is Node %s; extensions need %d or newer", v, MinMajor)
	}
	return v, nil
}

// ---------------------------------------------------------------- installing

// Plan says how InstallGlobal would install Node here.
type Plan struct {
	Method  string // "brew", "winget" or "download"
	Summary string // one line for the user
	Command string // the command run, when it is a package manager
	Bytes   int64  // download size, for "download"
}

func brewPath() string {
	if p, err := exec.LookPath("brew"); err == nil {
		return p
	}
	if !searchSystem {
		return ""
	}
	for _, p := range []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew", "/home/linuxbrew/.linuxbrew/bin/brew"} {
		if isFile(p) {
			return p
		}
	}
	return ""
}

// PlanInstall chooses the install method.
func PlanInstall() (Plan, error) {
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("winget"); err == nil {
			return Plan{Method: "winget", Summary: "Install Node.js LTS with winget",
				Command: "winget install --id OpenJS.NodeJS.LTS -e --silent --accept-package-agreements --accept-source-agreements"}, nil
		}
	} else if b := brewPath(); b != "" {
		verb := "install"
		if out, err := exec.Command(b, "list", "--versions", "node").Output(); err == nil && len(bytes.TrimSpace(out)) > 0 {
			verb = "upgrade"
		}
		return Plan{Method: "brew", Summary: "Install Node.js with Homebrew (brew " + verb + " node)", Command: b + " " + verb + " node"}, nil
	}
	if err := supported(); err != nil {
		return Plan{}, err
	}
	a := assets[platform()]
	where := "~/.local (node, npm and npx linked into ~/.local/bin)"
	if runtime.GOOS == "windows" {
		where = `%LOCALAPPDATA%\Programs\nodejs`
	}
	return Plan{Method: "download", Summary: fmt.Sprintf("Install Node.js %s (%d MB) to %s", Version, a.bytes>>20, where), Bytes: a.bytes}, nil
}

// Step reports install progress: a line of package-manager output, or a
// download's bytes (total > 0).
type Step func(line string, done, total int64)

// InstallGlobal installs Node for the account (see the package comment) and
// returns the Node it then finds.
func InstallGlobal(ctx context.Context, step Step) (Found, error) {
	if step == nil {
		step = func(string, int64, int64) {}
	}
	plan, err := PlanInstall()
	if err != nil {
		return Found{}, err
	}
	switch plan.Method {
	case "brew", "winget":
		f := strings.Fields(plan.Command)
		if err := run(ctx, step, f[0], f[1:]...); err != nil {
			return Found{}, fmt.Errorf("%s failed: %w", plan.Command, err)
		}
	default:
		if err := installDownload(ctx, step); err != nil {
			return Found{}, err
		}
	}
	Forget()
	f, err := Find()
	if err != nil {
		return Found{}, fmt.Errorf("Node.js was installed but cannot be found: %w", err)
	}
	return f, nil
}

// run streams a command's output, line by line, to step.
func run(ctx context.Context, step Step, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "HOMEBREW_NO_ENV_HINTS=1", "HOMEBREW_NO_INSTALL_CLEANUP=1", "NONINTERACTIVE=1")
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	var tail []string
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			l := strings.TrimSpace(sc.Text())
			if l == "" {
				continue
			}
			tail = append(tail, l)
			if len(tail) > 8 {
				tail = tail[1:]
			}
			step(l, 0, 0)
		}
	}()
	err := cmd.Wait()
	pw.Close()
	<-done
	if err != nil && len(tail) > 0 {
		return fmt.Errorf("%w: %s", err, tail[len(tail)-1])
	}
	return err
}

// localNodeRoot holds Node builds this package unpacked.
func localNodeRoot() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "lib", "nodejs")
}

func localBin() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
}

// installDownload unpacks the official build to ~/.local/lib/nodejs and
// links its commands into ~/.local/bin, leaving any file there that is not
// one of its own links alone.
func installDownload(ctx context.Context, step Step) error {
	a := assets[platform()]
	root := localNodeRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	if err := hithome.EnsureDirs(); err != nil {
		return err
	}
	if free := llmhost.FreeDisk(root); free > 0 && free < 4*a.bytes {
		return fmt.Errorf("not enough disk space for Node.js: %d MB free, about %d MB needed", free>>20, (4*a.bytes)>>20)
	}
	part, err := llmhost.DownloadVerified(ctx, "Node.js "+Version, origin+a.file, a.file, a.bytes, a.sum,
		func(_ string, done, total int64) { step("", done, total) })
	if err != nil {
		return err
	}
	defer os.Remove(part)
	step("Unpacking Node.js "+Version+"…", 0, 0)
	stage := filepath.Join(root, ".tmp-"+Version)
	_ = os.RemoveAll(stage)
	kind := "tar.gz"
	if strings.HasSuffix(a.file, ".zip") {
		kind = "zip"
	}
	if err := llmhost.ExtractArchive(kind, part, stage); err != nil {
		_ = os.RemoveAll(stage)
		return fmt.Errorf("unpacking Node.js: %w", err)
	}
	name := strings.TrimSuffix(strings.TrimSuffix(a.file, ".tar.gz"), ".zip")
	final := filepath.Join(root, name)
	_ = os.RemoveAll(final)
	if err := os.Rename(filepath.Join(stage, name), final); err != nil {
		_ = os.RemoveAll(stage)
		return err
	}
	_ = os.RemoveAll(stage)
	if runtime.GOOS == "windows" {
		step("Installed to "+final+" (add it to PATH to use node in your shell)", 0, 0)
		return nil
	}
	step("Linking node, npm and npx into ~/.local/bin…", 0, 0)
	if err := os.MkdirAll(localBin(), 0o755); err != nil {
		return err
	}
	for _, cmd := range []string{"node", "npm", "npx", "corepack"} {
		target := filepath.Join(final, "bin", cmd)
		if !exists(target) {
			continue
		}
		link := filepath.Join(localBin(), cmd)
		if cur, err := os.Readlink(link); err == nil {
			if !strings.HasPrefix(cur, root) {
				step("Kept your own ~/.local/bin/"+cmd, 0, 0)
				continue
			}
			_ = os.Remove(link)
		} else if exists(link) {
			step("Kept your own ~/.local/bin/"+cmd, 0, 0)
			continue
		}
		if err := os.Symlink(target, link); err != nil {
			return err
		}
	}
	if !onPath(localBin()) {
		step("~/.local/bin is not on your PATH; add it to use node in your shell", 0, 0)
	}
	return nil
}

// Remove deletes what this package installed itself (never a Homebrew or
// winget install) and reports what it removed.
func Remove() ([]string, error) {
	var removed []string
	root := localNodeRoot()
	ms, _ := filepath.Glob(filepath.Join(root, "node-v*"))
	if runtime.GOOS != "windows" {
		for _, cmd := range []string{"node", "npm", "npx", "corepack"} {
			link := filepath.Join(localBin(), cmd)
			if cur, err := os.Readlink(link); err == nil && strings.HasPrefix(cur, root) {
				if os.Remove(link) == nil {
					removed = append(removed, link)
				}
			}
		}
	}
	for _, m := range ms {
		if err := os.RemoveAll(m); err != nil {
			return removed, err
		}
		removed = append(removed, m)
	}
	if exists(hithome.Node()) {
		_ = os.RemoveAll(hithome.Node())
		removed = append(removed, hithome.Node())
	}
	Forget()
	return removed, nil
}

func onPath(dir string) bool {
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(d) == filepath.Clean(dir) {
			return true
		}
	}
	return false
}

// platform is the asset key, with one correction: an amd64 build of hittable
// running under Rosetta on Apple silicon should still get native arm64 Node.
func platform() string {
	arch := runtime.GOARCH
	if runtime.GOOS == "darwin" && arch == "amd64" {
		if out, err := exec.Command("sysctl", "-n", "sysctl.proc_translated").Output(); err == nil &&
			strings.TrimSpace(string(out)) == "1" {
			arch = "arm64"
		}
	}
	return runtime.GOOS + "/" + arch
}

// supported explains, before any download, why the official build would not
// run here: musl libc (Alpine) and glibc older than 2.28.
func supported() error {
	if _, ok := assets[platform()]; !ok {
		return fmt.Errorf("%w (%s); install Node %d+ with your package manager", ErrUnsupported, platform(), MinMajor)
	}
	if runtime.GOOS != "linux" {
		return nil
	}
	if isFile("/etc/alpine-release") || isMusl() {
		return fmt.Errorf("%w: this Linux uses musl libc (Alpine); install it with `apk add nodejs`", ErrUnsupported)
	}
	if maj, min, ok := glibcVersion(); ok && (maj < 2 || (maj == 2 && min < 28)) {
		return fmt.Errorf("%w: glibc %d.%d is older than the 2.28 Node %s needs; install Node with your package manager",
			ErrUnsupported, maj, min, Version)
	}
	return nil
}

func isMusl() bool {
	out, _ := exec.Command("ldd", "--version").CombinedOutput()
	return bytes.Contains(bytes.ToLower(out), []byte("musl"))
}

// glibcVersion parses the first line of `ldd --version`, e.g.
// "ldd (GNU libc) 2.31".
func glibcVersion() (major, minor int, ok bool) {
	out, err := exec.Command("ldd", "--version").Output()
	if err != nil {
		return 0, 0, false
	}
	f := strings.Fields(strings.SplitN(string(out), "\n", 2)[0])
	if len(f) == 0 {
		return 0, 0, false
	}
	parts := strings.SplitN(f[len(f)-1], ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	return major, minor, err1 == nil && err2 == nil
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }
