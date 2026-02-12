//go:build !linux

package client

import "fmt"

func waitWithTrace(pid int) (int, error) {
	_ = pid
	return 1, fmt.Errorf("lockdown tracing is only available on Linux")
}
