//go:build linux && amd64

package client

import (
	"fmt"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const maxTracedPathLen = 4096

func waitWithTrace(pid int) (int, error) {
	var ws syscall.WaitStatus

	// Initial stop from PTRACE_TRACEME before exec.
	if _, err := syscall.Wait4(pid, &ws, 0, nil); err != nil {
		return 1, fmt.Errorf("initial wait4: %w", err)
	}
	if ws.Exited() {
		return ws.ExitStatus(), nil
	}
	if ws.Signaled() {
		return 128 + int(ws.Signal()), nil
	}
	if !ws.Stopped() {
		return 1, fmt.Errorf("unexpected initial wait status: %v", ws)
	}

	if err := unix.PtraceSetOptions(pid, unix.PTRACE_O_TRACESYSGOOD|unix.PTRACE_O_EXITKILL); err != nil {
		_ = unix.Kill(pid, syscall.SIGKILL)
		return 1, fmt.Errorf("ptrace set options: %w", err)
	}

	inSyscall := false
	deliverSig := 0

	for {
		if err := unix.PtraceSyscall(pid, deliverSig); err != nil {
			_ = unix.Kill(pid, syscall.SIGKILL)
			return 1, fmt.Errorf("ptrace syscall continue: %w", err)
		}
		deliverSig = 0

		if _, err := syscall.Wait4(pid, &ws, 0, nil); err != nil {
			_ = unix.Kill(pid, syscall.SIGKILL)
			return 1, fmt.Errorf("wait4 traced child: %w", err)
		}

		if ws.Exited() {
			return ws.ExitStatus(), nil
		}
		if ws.Signaled() {
			return 128 + int(ws.Signal()), nil
		}
		if !ws.Stopped() {
			continue
		}

		sig := ws.StopSignal()
		syscallTrap := syscall.Signal(int(syscall.SIGTRAP) | 0x80)
		if sig == syscallTrap {
			// Syscall-enter and syscall-exit stop alternate under PTRACE_SYSCALL.
			if !inSyscall {
				path, err := forbiddenOpenPath(pid)
				if err != nil {
					_ = unix.Kill(pid, syscall.SIGKILL)
					return 1, fmt.Errorf("inspect syscall arguments: %w", err)
				}
				if path != "" {
					_ = unix.Kill(pid, syscall.SIGKILL)
					return 1, fmt.Errorf("security violation: child attempted forbidden access to %s", path)
				}
			}
			inSyscall = !inSyscall
			continue
		}

		// Non-syscall stop (e.g. SIGINT/SIGTERM): deliver it to traced child.
		if sig != syscall.SIGTRAP {
			deliverSig = int(sig)
		}
	}
}

func forbiddenOpenPath(pid int) (string, error) {
	var regs unix.PtraceRegs
	if err := unix.PtraceGetRegs(pid, &regs); err != nil {
		return "", err
	}

	sysno := int(regs.Orig_rax)
	var pathAddr uintptr
	switch sysno {
	case unix.SYS_OPEN:
		pathAddr = uintptr(regs.Rdi)
	case unix.SYS_OPENAT, unix.SYS_OPENAT2:
		pathAddr = uintptr(regs.Rsi)
	default:
		return "", nil
	}

	if pathAddr == 0 {
		return "", nil
	}

	path, err := readCString(pid, pathAddr, maxTracedPathLen)
	if err != nil {
		return "", err
	}
	if isForbiddenEnvironPath(path, pid) {
		return path, nil
	}
	return "", nil
}

func readCString(pid int, addr uintptr, maxLen int) (string, error) {
	if maxLen <= 0 {
		return "", nil
	}

	out := make([]byte, 0, 128)
	buf := make([]byte, 16)

	for len(out) < maxLen {
		n, err := unix.PtracePeekData(pid, addr+uintptr(len(out)), buf)
		if err != nil {
			return "", err
		}
		if n <= 0 {
			break
		}

		for i := 0; i < n && len(out) < maxLen; i++ {
			if buf[i] == 0 {
				return string(out), nil
			}
			out = append(out, buf[i])
		}
	}

	return string(out), nil
}

func isForbiddenEnvironPath(path string, pid int) bool {
	if path == "/proc/self/environ" || path == "/proc/thread-self/environ" {
		return true
	}
	if path == fmt.Sprintf("/proc/%d/environ", pid) {
		return true
	}
	// Block direct access to any proc environ file.
	return strings.HasPrefix(path, "/proc/") && strings.HasSuffix(path, "/environ")
}
