// Package llmhost downloads, verifies, installs and supervises llama-server —
// the sidecar that does the actual inference. It is the only package that
// touches the network on the user's behalf, the only one that spawns a child
// process, and the only one that knows a model has a size on disk.
//
// The shape of the contract is: nothing happens until someone asks. Preview
// measures what enabling would cost on this machine, Install does it,
// Installed is the cheap gate everything else polls, and Supervisor spawns the
// server lazily on the first request that needs one — never at startup.
//
// Non-goals: this package does not speak the inference protocol (that is
// internal/llm), does not know what a prompt or a commit message is, and owns
// no paths of its own (that is internal/hithome). It has no build tags and
// reads runtime.GOOS at run time, so an unsupported platform is a false
// Plan.Supported with a readable reason rather than a compile error. For the
// same reason free disk is measured by shelling out rather than through
// syscall.Statfs, which does not exist on every GOOS this module builds for.
package llmhost

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hittable/shellapp/internal/hithome"
)

// Errors callers branch on.
var (
	// ErrUnsupported means llama.cpp publishes no prebuilt CPU runtime for this
	// GOOS/GOARCH. It is reported through Plan.Supported, never as a panic.
	ErrUnsupported = errors.New("llmhost: unsupported platform")
	// ErrUnsafeArchive means an archive entry tried to write outside the
	// destination. The archive is discarded; nothing is installed.
	ErrUnsafeArchive = errors.New("llmhost: archive entry escapes the destination")
	// ErrChecksum means a completed download did not match its pinned hash.
	// This is executable code, so the artifact is deleted rather than kept.
	ErrChecksum = errors.New("llmhost: checksum mismatch")
	// ErrNotInstalled means a server was asked for before anything was
	// downloaded.
	ErrNotInstalled = errors.New("llmhost: nothing installed")
	// ErrRestartBudget means the server has failed to stay up too many times in
	// a row. A model that OOMs on every load must not become a spawn loop.
	ErrRestartBudget = errors.New("llmhost: too many restarts")
)

// Plan is what `enable` would do, measured on this machine. Every number here
// comes from the machine or the pinned artifact table; none is a guess the user
// cannot check, except InstallBytes, which estimates the extracted size of an
// archive that has not been downloaded yet.
type Plan struct {
	Supported   bool
	Unsupported string // reason when !Supported

	RuntimeName  string
	RuntimeBytes int64
	ModelName    string
	ModelBytes   int64

	// DownloadBytes is what still has to come over the network: it excludes
	// anything already present and verified, so re-running enable after an
	// interruption reports only the remainder.
	DownloadBytes int64
	InstallBytes  int64 // on disk once installed
	RAMBytes      int64 // approximate resident size while generating

	FreeDisk int64 // on the filesystem holding the install target, 0 if unknown
	TotalRAM int64 // physical memory, 0 if unknown

	RuntimePresent bool // installed and matching the pinned hash
	ModelPresent   bool // present at exactly the pinned size
}

// runtimeExpansion estimates the extracted size of the runtime archive as a
// multiple of the compressed size. It only moves the total by tens of megabytes
// against a two-gigabyte model, so a measured value is not worth a round trip;
// once the runtime is installed the real size is used instead.
const runtimeExpansion = 3

// Preview measures what Install would do, without doing any of it and without
// touching the network.
func Preview() (Plan, error) { return preview(thisOS(), thisArch()) }

func preview(goos, goarch string) (Plan, error) {
	p := Plan{
		ModelName:  ModelLabel,
		ModelBytes: ModelDownloadBytes,
		RAMBytes:   ModelRAMBytes,
		FreeDisk:   freeDisk(hithome.Dir()),
		TotalRAM:   totalRAM(),
	}

	a, err := runtimeArtifact(goos, goarch)
	if err != nil {
		p.Unsupported = fmt.Sprintf("llama.cpp publishes no prebuilt runtime for %s", platformKey(goos, goarch))
		return p, nil
	}
	p.Supported = true
	p.RuntimeName = a.label
	p.RuntimeBytes = a.bytes

	p.RuntimePresent = runtimeVerified(a.sum, goos)
	p.ModelPresent = modelPresent()

	if !p.RuntimePresent {
		p.DownloadBytes += a.bytes
	}
	if !p.ModelPresent {
		p.DownloadBytes += ModelDownloadBytes
	}

	runtimeOnDisk := a.bytes * runtimeExpansion
	if p.RuntimePresent {
		if n, err := dirSize(hithome.RuntimeTag(RuntimeTag)); err == nil {
			runtimeOnDisk = n
		}
	}
	p.InstallBytes = runtimeOnDisk + ModelDownloadBytes
	return p, nil
}

// Resource thresholds. These are policy, kept here so the CLI and any future
// TUI affordance refuse on identical terms rather than drifting apart.
const (
	// diskHeadroom is applied to InstallBytes: refusing at 1.3× is far kinder
	// than dying at 94% of a two-gigabyte download.
	diskHeadroomNum, diskHeadroomDen = 13, 10
	minRAM                           = 4 << 30
	comfortableRAM                   = 8 << 30
)

// Check applies the resource policy to a plan. A non-nil error means do not
// start; a non-empty warning means it will work but is likely to swap, and
// deserves an explicit yes. An unknown FreeDisk or TotalRAM (0) is not treated
// as a failure — refusing because a measurement is unavailable would be worse
// than letting the download try.
func (p Plan) Check() (warn string, err error) {
	if !p.Supported {
		return "", fmt.Errorf("%w: %s", ErrUnsupported, p.Unsupported)
	}
	need := p.InstallBytes * diskHeadroomNum / diskHeadroomDen
	if p.FreeDisk > 0 && p.FreeDisk < need {
		return "", fmt.Errorf("not enough free disk: %d bytes available, %d needed including headroom", p.FreeDisk, need)
	}
	if p.TotalRAM > 0 && p.TotalRAM < minRAM {
		return "", fmt.Errorf("this machine has %d bytes of memory; the model needs about %d to run", p.TotalRAM, p.RAMBytes)
	}
	if p.TotalRAM > 0 && p.TotalRAM < comfortableRAM {
		return fmt.Sprintf("%d bytes of memory total; generating will use about %d and is likely to swap",
			p.TotalRAM, p.RAMBytes), nil
	}
	return "", nil
}

// State is what `status` reports.
type State struct {
	RuntimeInstalled bool
	RuntimeTag       string
	RuntimeBytes     int64

	ModelInstalled bool
	ModelFile      string
	ModelBytes     int64

	TotalBytes int64 // everything under the home directory

	ServerRunning bool
	ServerPID     int
	ServerPort    int

	// Endpoint is non-empty only when the user configured an external server,
	// in which case nothing is ever downloaded or spawned.
	Endpoint string
}

// Status inspects the install. It never fails on a missing directory — nothing
// installed is a valid state, not an error.
func Status() (State, error) {
	st := State{RuntimeTag: RuntimeTag, ModelFile: ModelFileName}

	if fi, err := os.Stat(ServerPath()); err == nil && !fi.IsDir() {
		st.RuntimeInstalled = true
		st.RuntimeBytes, _ = dirSize(hithome.RuntimeTag(RuntimeTag))
	}
	if fi, err := os.Stat(ModelPath()); err == nil && !fi.IsDir() {
		st.ModelInstalled = true
		st.ModelBytes = fi.Size()
	}
	st.TotalBytes, _ = dirSize(hithome.Dir())

	if rs, err := readRunState(); err == nil && processAlive(rs.PID) {
		st.ServerRunning = true
		st.ServerPID = rs.PID
		st.ServerPort = rs.Port
	}
	st.Endpoint = configuredEndpoint()
	return st, nil
}

// ServerPath is where the supervised binary lives once installed.
func ServerPath() string {
	return filepath.Join(hithome.RuntimeTag(RuntimeTag), serverBinName(thisOS()))
}

// ModelPath is where the weights live once installed.
func ModelPath() string { return hithome.Model(ModelFileName) }

// Installed is the cheap gate the TUI polls every two seconds: two stats, no
// hashing, no directory walk, no network. It answers "is there something to
// use", not "is it byte-for-byte the artifact we pinned" — Preview answers
// that, and only when the user is about to act on it.
func Installed() bool {
	if fi, err := os.Stat(ServerPath()); err != nil || fi.IsDir() {
		return false
	}
	fi, err := os.Stat(ModelPath())
	return err == nil && !fi.IsDir()
}

// modelPresent is Installed's model half, plus the size check Preview can
// afford. Hashing two gigabytes to render a confirmation prompt cannot be
// justified; a truncated download has the wrong size, and a corrupted one is
// caught by the hash at install time, before anything is renamed into place.
func modelPresent() bool { return modelPresentSized(ModelDownloadBytes) }

func modelPresentSized(want int64) bool {
	fi, err := os.Stat(ModelPath())
	return err == nil && !fi.IsDir() && fi.Size() == want
}

// markerFile records the hash of the archive a runtime directory was extracted
// from. Without it "already installed" could only mean "a directory exists",
// which would silently keep a half-extracted or superseded build.
const markerFile = ".sha256"

func runtimeVerified(sum, goos string) bool {
	b, err := os.ReadFile(filepath.Join(hithome.RuntimeTag(RuntimeTag), markerFile))
	if err != nil || strings.TrimSpace(string(b)) != sum {
		return false
	}
	fi, err := os.Stat(filepath.Join(hithome.RuntimeTag(RuntimeTag), serverBinName(goos)))
	return err == nil && !fi.IsDir()
}

// Stop shuts down a running llama-server and reports whether one was there,
// along with the memory it was holding. The model stays on disk: this is the
// difference between "I am not using it right now" and "take it off my
// machine", and conflating the two means a user who wants their RAM back pays
// a two-gigabyte download to get the feature working again.
//
// It stops the server recorded in the run file, so it works from the CLI on a
// server a running TUI started.
func Stop() (wasRunning bool, freedRAM int64, err error) {
	rs, readErr := readRunState()
	if readErr != nil || !processAlive(rs.PID) {
		_ = os.Remove(hithome.RunFile())
		return false, 0, nil
	}
	freedRAM = residentBytes(rs.PID)
	stopRunFileServer()
	return true, freedRAM, nil
}

// residentBytes asks ps for a pid's resident set size. It is for a human-facing
// number only, so an unparsable answer reports the model's expected footprint
// rather than failing the command.
func residentBytes(pid int) int64 {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err == nil {
		if kb, convErr := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); convErr == nil && kb > 0 {
			return kb * 1024
		}
	}
	return ModelRAMBytes
}

// Remove stops any server this app started, deletes everything it downloaded,
// and reports the bytes reclaimed. config.json is deliberately kept: the user's
// settings are not part of what `disable` was asked to remove.
func Remove() (freed int64, err error) {
	stopRunFileServer()

	var last error
	for _, dir := range []string{
		hithome.Runtime(), hithome.Models(), hithome.Run(), hithome.Tmp(), hithome.Logs(),
	} {
		n, sizeErr := dirSize(dir)
		if sizeErr != nil {
			continue // it was never there
		}
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			last = rmErr
			continue
		}
		freed += n
	}
	return freed, last
}

// dirSize sums the regular files under dir. Symlinks are counted as their own
// (tiny) size rather than followed, so a link into the model directory cannot
// double-count two gigabytes.
func dirSize(dir string) (int64, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return 0, err
	}
	if !fi.IsDir() {
		return fi.Size(), nil
	}
	var total int64
	err = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable corner is not worth failing a size report
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, err
}
