package llmhost

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hittable/shellapp/internal/appconfig"
)

// Everything in this file is best-effort by design: a machine that will not
// report its free memory is still a machine the model can run on, and a
// codesign that fails is only fatal if the binary then refuses to start — which
// the smoke test is there to discover, with a real error.

// shellTimeout bounds the small helper commands below. None of them should take
// more than a few milliseconds; a hung `df` on a stale network mount must not
// hang a confirmation prompt.
const shellTimeout = 3 * time.Second

func output(name string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), shellTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

func quiet(name string, args ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), shellTimeout)
	defer cancel()
	_ = exec.CommandContext(ctx, name, args...).Run()
}

// freeDisk reports the bytes available on the filesystem holding dir, or 0 when
// it cannot be determined.
//
// This shells out rather than calling syscall.Statfs because Statfs does not
// exist on Windows, and this package must compile for every GOOS the module
// builds for without introducing a build tag or promoting x/sys to a direct
// dependency. Shelling out is already the house habit for exactly this reason
// (gitx.Run, the uninstall pgrep sweep).
func freeDisk(dir string) int64 {
	dir = nearestExisting(dir)
	if dir == "" {
		return 0
	}
	if runtime.GOOS == "windows" {
		return freeDiskWindows(dir)
	}
	// -P forces the POSIX one-line-per-filesystem format; without it a long
	// device name wraps and the columns move.
	out, ok := output("df", "-Pk", dir)
	if !ok {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return 0
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return 0
	}
	blocks, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil || blocks < 0 {
		return 0
	}
	return blocks * 1024
}

func freeDiskWindows(dir string) int64 {
	// Single-quote escaping for a PowerShell literal is doubling the quote.
	lit := strings.ReplaceAll(dir, "'", "''")
	out, ok := output("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"(Get-PSDrive -Name (Get-Item -LiteralPath '"+lit+"').PSDrive.Name).Free")
	if !ok {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// nearestExisting walks up from dir until it finds something that exists, so
// free space can be measured for a directory that has not been created yet.
func nearestExisting(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	for {
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return ""
		}
		abs = parent
	}
}

// totalRAM reports physical memory in bytes, or 0 when unknown.
func totalRAM() int64 {
	switch runtime.GOOS {
	case "darwin":
		out, ok := output("sysctl", "-n", "hw.memsize")
		if !ok {
			return 0
		}
		n, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
		if err != nil {
			return 0
		}
		return n
	case "linux":
		b, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0
		}
		for _, line := range strings.Split(string(b), "\n") {
			rest, ok := strings.CutPrefix(line, "MemTotal:")
			if !ok {
				continue
			}
			fields := strings.Fields(rest)
			if len(fields) == 0 {
				return 0
			}
			kb, err := strconv.ParseInt(fields[0], 10, 64)
			if err != nil {
				return 0
			}
			return kb * 1024
		}
	}
	return 0
}

// processAlive reports whether a pid names a live process. Signal 0 is the
// portable liveness probe on unix; Windows does not implement it, so a process
// os.FindProcess could open is taken as alive there rather than reporting every
// running server as dead.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	switch {
	case err == nil:
		return true
	case errors.Is(err, os.ErrProcessDone), errors.Is(err, syscall.ESRCH):
		return false
	default:
		// EPERM (someone else's process) or "not supported" (Windows): it
		// exists, we just may not signal it.
		return true
	}
}

// postProcess makes an extracted runtime actually runnable on this platform.
// Every step is non-fatal: if one of these is what the install needed, the
// smoke test fails with a real error, which is far more useful than a failed
// `xattr` aborting an otherwise good download.
func postProcess(dir, bin string) {
	if runtime.GOOS != "windows" {
		chmodExecutables(dir)
	}
	if runtime.GOOS != "darwin" {
		return
	}
	// A Go net/http download never gets com.apple.quarantine — Go does not set
	// it — but the attribute may arrive some other way, and removing it costs
	// nothing. The real problem on arm64 is that an unsigned Mach-O is killed
	// by the kernel, which is what the ad-hoc signature below fixes; the
	// Makefile does the same thing to this app's own binary for the same
	// reason.
	quiet("xattr", "-dr", "com.apple.quarantine", dir)

	// Libraries first, executable second. An executable's signature covers the
	// libraries it loads, so re-signing a dylib afterwards would invalidate the
	// signature that had just been applied to the binary.
	for _, lib := range sharedLibs(dir) {
		quiet("codesign", "--force", "--sign", "-", lib)
	}
	quiet("codesign", "--force", "--sign", "-", bin)
}

// sharedLibs lists the dynamic libraries shipped alongside the binary.
func sharedLibs(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if isSharedLib(d.Name()) {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func isSharedLib(name string) bool {
	return strings.HasSuffix(name, ".dylib") ||
		strings.HasSuffix(name, ".so") ||
		strings.Contains(name, ".so.") ||
		strings.HasSuffix(name, ".dll")
}

// chmodExecutables restores the executable bit. A zip carries no unix mode at
// all, and a tar written on a build machine may not carry the one we want.
func chmodExecutables(dir string) {
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		// Binaries in these archives have no extension; libraries are the other
		// thing that must be readable and executable.
		if strings.Contains(name, ".") && !isSharedLib(name) {
			return nil
		}
		_ = os.Chmod(p, 0o755)
		return nil
	})
}

// libraryEnv returns the environment a spawned server needs to find the shared
// libraries shipped next to it. The published llama.cpp builds carry no RPATH
// guarantee, so without this the server starts and immediately dies looking for
// libllama.
func libraryEnv(base []string, dir string) []string {
	var key string
	switch runtime.GOOS {
	case "darwin":
		key = "DYLD_LIBRARY_PATH"
	case "linux":
		key = "LD_LIBRARY_PATH"
	default:
		return base
	}
	value := dir
	if prev := os.Getenv(key); prev != "" {
		value = dir + string(os.PathListSeparator) + prev
	}
	return append(base, key+"="+value)
}

// configuredEndpoint reports an externally configured server, if any. A
// non-empty value means this package must never download or spawn anything.
func configuredEndpoint() string {
	cfg, _ := appconfig.Load("")
	return strings.TrimSpace(cfg.AI.Endpoint)
}
