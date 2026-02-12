//go:build !linux

package client

import "os/exec"

func setProcAttr(cmd *exec.Cmd, lockdown bool) {
	// No platform-specific process hardening on non-Linux
	_ = lockdown
	_ = cmd
}
