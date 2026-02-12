//go:build linux

package client

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// LockdownExec applies security hardening, then replaces this process with the target
// command via execve. Intended to run inside a child process spawned by the runner.
//
// Hardening applied:
//   - PR_SET_NO_NEW_PRIVS: required for seccomp without CAP_SYS_ADMIN
//   - Seccomp BPF filter: blocks ptrace(2) and process_vm_readv(2) syscalls.
//     Since seccomp filters are inherited across execve, the target binary
//     cannot be traced or have its memory read via these syscalls.
//   - PR_SET_DUMPABLE=0: set right before exec (resets after exec, but
//     combined with seccomp provides defense-in-depth during the gap)
func LockdownExec(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("_exec: no command specified")
	}

	// Seccomp filter: block ptrace and process_vm_readv.
	// Fail closed if hardening cannot be applied.
	if err := installSeccompFilter(); err != nil {
		return fmt.Errorf("_exec: install seccomp filter: %w", err)
	}

	if err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0); err != nil {
		return fmt.Errorf("_exec: PR_SET_DUMPABLE: %w", err)
	}

	binary, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("_exec: command not found: %s", args[0])
	}

	return syscall.Exec(binary, args, os.Environ())
}

// installSeccompFilter installs a BPF filter that blocks ptrace(2) and
// process_vm_readv(2) with EPERM. All other syscalls are allowed.
// The filter survives execve.
func installSeccompFilter() error {
	const (
		seccompRetAllow = 0x7fff0000
		seccompRetErrno = 0x00050000 // SECCOMP_RET_ERRNO
	)

	sysNumPtrace := uint32(unix.SYS_PTRACE)
	sysNumProcessVmReadv := uint32(unix.SYS_PROCESS_VM_READV)

	filter := []unix.SockFilter{
		// Load syscall number (offset 0 in seccomp_data)
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0},
		// if nr == ptrace → goto deny
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: sysNumPtrace, Jt: 2},
		// if nr == process_vm_readv → goto deny
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: sysNumProcessVmReadv, Jt: 1},
		// allow
		{Code: unix.BPF_RET | unix.BPF_K, K: seccompRetAllow},
		// deny: return EPERM (errno 1)
		{Code: unix.BPF_RET | unix.BPF_K, K: seccompRetErrno | 1},
	}

	prog := unix.SockFprog{
		Len:    uint16(len(filter)),
		Filter: &filter[0],
	}

	// Required before SECCOMP_SET_MODE_FILTER without CAP_SYS_ADMIN
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("PR_SET_NO_NEW_PRIVS: %w", err)
	}

	_, _, errno := unix.RawSyscall(
		unix.SYS_SECCOMP,
		1, // SECCOMP_SET_MODE_FILTER
		0, // flags
		uintptr(unsafe.Pointer(&prog)),
	)
	if errno != 0 {
		return fmt.Errorf("SECCOMP_SET_MODE_FILTER: %v", errno)
	}

	return nil
}
