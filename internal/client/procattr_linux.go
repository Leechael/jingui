//go:build linux

package client

import (
	"os/exec"
	"runtime"
	"syscall"
)

func setProcAttr(cmd *exec.Cmd, lockdown bool) {
	attr := &syscall.SysProcAttr{
		// Kill child when parent dies (prevents orphan processes holding secrets)
		Pdeathsig: syscall.SIGKILL,
	}
	// Ptrace-based syscall inspection is currently implemented on amd64 only.
	if lockdown && runtime.GOARCH == "amd64" {
		attr.Ptrace = true
	}
	cmd.SysProcAttr = attr
}
