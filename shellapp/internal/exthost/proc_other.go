//go:build !linux && !windows

package exthost

import "syscall"

// Only Linux has a parent-death signal; elsewhere the host exits when its
// stdin closes, which happens when hittable exits for any reason.
func setDeathSignal(*syscall.SysProcAttr) {}
