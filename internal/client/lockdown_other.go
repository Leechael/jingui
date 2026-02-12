//go:build !linux

package client

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// LockdownExec on non-Linux just performs a plain execve.
func LockdownExec(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("_exec: no command specified")
	}

	binary, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("_exec: command not found: %s", args[0])
	}

	return syscall.Exec(binary, args, os.Environ())
}
