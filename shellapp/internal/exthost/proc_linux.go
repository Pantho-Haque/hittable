//go:build linux

package exthost

import "syscall"

func setDeathSignal(a *syscall.SysProcAttr) { a.Pdeathsig = syscall.SIGTERM }
