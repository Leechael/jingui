package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/aspect-build/jingui/internal/client"
	"github.com/aspect-build/jingui/internal/crypto"
	"github.com/aspect-build/jingui/internal/refparser"
	"github.com/aspect-build/jingui/internal/version"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/curve25519"
)

// devCommands is populated by dev.go (build tag "dev") with dev-only subcommands.
var devCommands []*cobra.Command

// resolveServerURL returns the server URL from the flag or JINGUI_SERVER_URL env var.
// Prints a warning to stderr when falling back to the env var.
// Returns an error if neither is set.
func resolveServerURL(cmd *cobra.Command, flagValue string) (string, error) {
	normalize := func(v string) string {
		for len(v) > 0 && v[len(v)-1] == '/' {
			v = v[:len(v)-1]
		}
		return v
	}
	if cmd.Flags().Changed("server") {
		return normalize(flagValue), nil
	}
	if v := os.Getenv("JINGUI_SERVER_URL"); v != "" {
		fmt.Fprintf(os.Stderr, "jingui: WARNING: using server URL from JINGUI_SERVER_URL environment variable\n")
		return normalize(v), nil
	}
	return "", fmt.Errorf("server URL required: use --server flag or set JINGUI_SERVER_URL")
}

const defaultAppkeysPath = "/dstack/.host-shared/.appkeys.json"

func main() {
	rootCmd := &cobra.Command{
		Use:     "jingui",
		Short:   "Jingui (金匮) - secure secret injection for TEE environments",
		Version: version.Version,
	}
	rootCmd.SetVersionTemplate(version.String("jingui") + "\n")

	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newReadCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newExecCmd())
	for _, cmd := range devCommands {
		rootCmd.AddCommand(cmd)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRunCmd() *cobra.Command {
	var (
		envFile     string
		serverURL   string
		appkeysPath string
		insecure    bool
		noLockdown  bool
	)

	cmd := &cobra.Command{
		Use:   "run [flags] -- <command> [args...]",
		Short: "Run a command with resolved secrets and output masking",
		Long: `Resolve jingui:// references in the env file, fetch and decrypt secrets
from the jingui server, then launch the command with the real values injected.
All secret values are masked in stdout/stderr with [REDACTED_BY_JINGUI].`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveServerURL(cmd, serverURL)
			if err != nil {
				return err
			}
			envFileExplicit := cmd.Flags().Changed("env-file")
			return runCommand(envFile, envFileExplicit, resolved, appkeysPath, insecure, !noLockdown, args)
		},
	}

	cmd.Flags().StringVar(&envFile, "env-file", ".env", "Path to .env file (skipped if not found and not explicitly set)")
	cmd.Flags().StringVar(&serverURL, "server", "", "Jingui server URL (or set JINGUI_SERVER_URL)")
	cmd.Flags().StringVar(&appkeysPath, "appkeys", defaultAppkeysPath, "Path to appkeys file")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Allow plaintext HTTP connection to server")
	cmd.Flags().BoolVar(&noLockdown, "no-lockdown", false, "Disable seccomp/ptrace lockdown on child process")

	return cmd
}

func newReadCmd() *cobra.Command {
	var (
		serverURL   string
		appkeysPath string
		insecure    bool
		showMeta    bool
	)

	cmd := &cobra.Command{
		Use:   "read <secret_ref>",
		Short: "Read a single secret reference (for debugging)",
		Long: `Fetch and decrypt a single jingui:// secret reference.
Prints the plaintext value to stdout. Useful for verifying connectivity.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveServerURL(cmd, serverURL)
			if err != nil {
				return err
			}
			return readSecret(resolved, appkeysPath, insecure, showMeta, args[0])
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", "", "Jingui server URL (or set JINGUI_SERVER_URL)")
	cmd.Flags().StringVar(&appkeysPath, "appkeys", defaultAppkeysPath, "Path to appkeys file")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Allow plaintext HTTP connection to server")
	cmd.Flags().BoolVar(&showMeta, "show-meta", false, "Print FID/Public Key to stderr for debugging")

	return cmd
}

func newStatusCmd() *cobra.Command {
	var (
		serverURL   string
		appkeysPath string
		insecure    bool
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show local instance status and server registration",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveServerURL(cmd, serverURL)
			if err != nil {
				return err
			}
			return showStatus(resolved, appkeysPath, insecure)
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", "", "Jingui server URL (or set JINGUI_SERVER_URL)")
	cmd.Flags().StringVar(&appkeysPath, "appkeys", defaultAppkeysPath, "Path to appkeys file")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Allow plaintext HTTP connection to server")

	return cmd
}

// newExecCmd creates the hidden _exec subcommand used by the runner to apply
// seccomp/PR_SET_DUMPABLE before execve into the target binary.
func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "_exec -- <command> [args...]",
		Hidden: true,
		Args:   cobra.MinimumNArgs(1),
		// DisableFlagParsing so that everything after _exec is passed as args
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Strip leading "--" if present
			if len(args) > 0 && args[0] == "--" {
				args = args[1:]
			}
			return client.LockdownExec(args)
		},
	}
	return cmd
}

func runCommand(envFile string, envFileExplicit bool, serverURL, appkeysPath string, insecure, lockdown bool, command []string) error {
	privKey, err := client.LoadPrivateKey(appkeysPath)
	if err != nil {
		return fmt.Errorf("load private key: %w", err)
	}

	fid, err := client.ComputeFID(privKey)
	if err != nil {
		return fmt.Errorf("compute FID: %w", err)
	}

	// Parse env file — if --env-file was not explicitly set and the default
	// doesn't exist, just scan the current process environment.
	var entries []client.EnvEntry
	entries, err = client.ParseEnvFile(envFile)
	if err != nil {
		if !envFileExplicit && errors.Is(err, os.ErrNotExist) {
			// Default .env not found → proceed without it
			entries = nil
		} else {
			return fmt.Errorf("parse env file: %w", err)
		}
	}

	scan := client.MergeEnvFileWithProcess(entries)

	// Lockdown only meaningful on Linux
	effectiveLockdown := lockdown && runtime.GOOS == "linux"
	if effectiveLockdown && runtime.GOARCH != "amd64" {
		return fmt.Errorf("lockdown is currently supported only on linux/amd64; use --no-lockdown to bypass (not recommended)")
	}

	if len(scan.Refs) == 0 {
		cfg := client.RunConfig{
			Command:  command[0],
			Args:     command[1:],
			Env:      scan.PlainEnv,
			Lockdown: effectiveLockdown,
		}
		exitCode, err := client.Run(cfg)
		if err != nil {
			return err
		}
		os.Exit(exitCode)
	}

	refList := make([]string, 0, len(scan.Refs))
	seen := make(map[string]bool)
	for _, ref := range scan.Refs {
		if !seen[ref] {
			refList = append(refList, ref)
			seen[ref] = true
		}
	}

	blobs, err := client.Fetch(serverURL, privKey, fid, refList, insecure, "run")
	if err != nil {
		return fmt.Errorf("fetch secrets: %w", err)
	}

	resolved := make(map[string]string)
	var secretValues []string

	for ref, blob := range blobs {
		plain, err := crypto.Decrypt(privKey, blob)
		if err != nil {
			return fmt.Errorf("decrypt secret %s: %w", ref, err)
		}
		resolved[ref] = string(plain)
		secretValues = append(secretValues, string(plain))
	}

	env := make([]string, len(scan.PlainEnv))
	copy(env, scan.PlainEnv)
	for key, ref := range scan.Refs {
		val, ok := resolved[ref]
		if !ok {
			return fmt.Errorf("missing resolved value for %s", ref)
		}
		env = append(env, key+"="+val)
	}

	cfg := client.RunConfig{
		Command:  command[0],
		Args:     command[1:],
		Env:      env,
		Secrets:  secretValues,
		Lockdown: effectiveLockdown,
	}

	exitCode, err := client.Run(cfg)
	if err != nil {
		return err
	}
	os.Exit(exitCode)
	return nil
}

func showStatus(serverURL, appkeysPath string, insecure bool) error {
	privKey, err := client.LoadPrivateKey(appkeysPath)
	if err != nil {
		return fmt.Errorf("load private key: %w", err)
	}

	fid, err := client.ComputeFID(privKey)
	if err != nil {
		return fmt.Errorf("compute FID: %w", err)
	}

	pub, _ := curve25519.X25519(privKey[:], curve25519.Basepoint)
	fmt.Printf("appkeys_path=%s\n", appkeysPath)
	fmt.Printf("fid=%s\n", fid)
	fmt.Printf("public_key=%s\n", hex.EncodeToString(pub))

	if err := client.CheckInstance(serverURL, fid, insecure); err != nil {
		fmt.Printf("server=%s\n", serverURL)
		fmt.Printf("registered=false\n")
		fmt.Printf("status_error=%v\n", err)
		return nil
	}

	fmt.Printf("server=%s\n", serverURL)
	fmt.Printf("registered=true\n")
	return nil
}

func readSecret(serverURL, appkeysPath string, insecure, showMeta bool, secretRef string) error {
	if _, err := refparser.Parse(secretRef); err != nil {
		return fmt.Errorf("invalid secret reference: %w", err)
	}

	privKey, err := client.LoadPrivateKey(appkeysPath)
	if err != nil {
		return fmt.Errorf("load private key: %w", err)
	}

	fid, err := client.ComputeFID(privKey)
	if err != nil {
		return fmt.Errorf("compute FID: %w", err)
	}

	if showMeta {
		pub, _ := curve25519.X25519(privKey[:], curve25519.Basepoint)
		fmt.Fprintf(os.Stderr, "FID: %s\n", fid)
		fmt.Fprintf(os.Stderr, "Public Key: %s\n", hex.EncodeToString(pub))
	}

	blobs, err := client.Fetch(serverURL, privKey, fid, []string{secretRef}, insecure, "read")
	if err != nil {
		return fmt.Errorf("fetch secret: %w", err)
	}

	blob, ok := blobs[secretRef]
	if !ok {
		return fmt.Errorf("server did not return requested secret")
	}

	plain, err := crypto.Decrypt(privKey, blob)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	fmt.Print(string(plain))
	return nil
}
