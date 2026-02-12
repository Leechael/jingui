package client

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// RunConfig holds the configuration for launching a subprocess with secret masking.
type RunConfig struct {
	Command  string
	Args     []string
	Env      []string
	Secrets  []string // Secret values to mask in stdout/stderr
	Lockdown bool     // If true, spawn via "jingui _exec" for seccomp hardening
}

// Run launches a subprocess with masked stdout/stderr, forwarding signals.
// Returns the child process exit code.
func Run(cfg RunConfig) (int, error) {
	var cmd *exec.Cmd

	if cfg.Lockdown {
		// Spawn via "jingui _exec -- <command> [args...]" to apply seccomp
		// and PR_SET_DUMPABLE before execve into the target binary.
		self, err := os.Executable()
		if err != nil {
			return 1, fmt.Errorf("resolve self executable: %w", err)
		}
		execArgs := append([]string{"_exec", "--"}, cfg.Command)
		execArgs = append(execArgs, cfg.Args...)
		cmd = exec.Command(self, execArgs...)
	} else {
		cmd = exec.Command(cfg.Command, cfg.Args...)
	}

	cmd.Env = cfg.Env
	cmd.Stdin = os.Stdin

	// Platform-specific process attributes (e.g. Pdeathsig on Linux)
	setProcAttr(cmd, cfg.Lockdown)

	stdoutMasker := NewMaskingWriter(os.Stdout, cfg.Secrets)
	stderrMasker := NewMaskingWriter(os.Stderr, cfg.Secrets)
	cmd.Stdout = stdoutMasker
	cmd.Stderr = stderrMasker

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("start command: %w", err)
	}

	go func() {
		for sig := range sigCh {
			_ = cmd.Process.Signal(sig)
		}
	}()

	var (
		exitCode int
		err      error
	)

	if cfg.Lockdown {
		exitCode, err = waitWithTrace(cmd.Process.Pid)
	} else {
		err = cmd.Wait()
		if err == nil {
			exitCode = 0
		}
	}

	_ = stdoutMasker.Flush()
	_ = stderrMasker.Flush()

	if err != nil {
		if !cfg.Lockdown {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode(), nil
			}
		}
		return 1, fmt.Errorf("wait command: %w", err)
	}

	return exitCode, nil
}
