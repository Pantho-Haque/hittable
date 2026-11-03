package extensions

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// TargetPlatform is this machine in the marketplace's vocabulary
// (darwin-arm64, linux-x64, alpine-x64, win32-x64, …). Extensions that ship a
// native binary, such as rust-analyzer, publish one .vsix per target.
func TargetPlatform() string {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64", "arm": "armhf"}[runtime.GOARCH]
	if arch == "" {
		arch = runtime.GOARCH
	}
	switch runtime.GOOS {
	case "windows":
		return "win32-" + arch
	case "darwin":
		// An amd64 hittable under Rosetta still runs native arm64 Node, so it
		// needs the arm64 build of any bundled binary.
		if arch == "x64" {
			if out, err := exec.Command("sysctl", "-n", "sysctl.proc_translated").Output(); err == nil &&
				strings.TrimSpace(string(out)) == "1" {
				arch = "arm64"
			}
		}
		return "darwin-" + arch
	case "linux":
		if _, err := os.Stat("/etc/alpine-release"); err == nil {
			return "alpine-" + arch
		}
		return "linux-" + arch
	}
	return runtime.GOOS + "-" + arch
}

// platformFits reports whether a build for target runs here. "universal"
// and "" fit everywhere; "web" builds have no Node entry point.
func platformFits(target, here string) bool {
	return target == "" || target == "universal" || target == here
}
