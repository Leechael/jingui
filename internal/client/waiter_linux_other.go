//go:build linux && !amd64

package client

import (
	"fmt"
)

// On non-amd64 Linux we fail closed for lockdown mode: ptrace syscall-path
// inspection is currently implemented on amd64 only.
func waitWithTrace(pid int) (int, error) {
	_ = pid
	return 1, fmt.Errorf("lockdown is currently supported only on linux/amd64; use --no-lockdown to bypass (not recommended)")
}
