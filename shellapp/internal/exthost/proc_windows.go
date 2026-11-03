//go:build windows

package exthost

import (
	"os/exec"
	"strconv"
)

func setProcAttrs(*exec.Cmd) {}

// killTree uses taskkill /T, which also ends the children (language
// servers) Windows does not tie to the parent.
func killTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}
