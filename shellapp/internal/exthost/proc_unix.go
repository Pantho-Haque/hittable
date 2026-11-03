//go:build !windows

package exthost

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcAttrs puts the host in its own process group, so killTree also
// ends the language servers extensions spawn, and (on Linux) has the kernel
// kill it if hittable dies without cleaning up.
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	setDeathSignal(cmd.SysProcAttr)
}

func killTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pgid := cmd.Process.Pid
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	go func() {
		time.Sleep(1500 * time.Millisecond)
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}()
}
