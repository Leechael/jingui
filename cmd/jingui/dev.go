//go:build dev

package main

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/curve25519"
)

func init() {
	devCommands = append(devCommands, newInitCmd())
}

func newInitCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "[dev] Generate a new .appkeys.json (X25519 keypair) and print registration info",
		Long: `Generate a fresh X25519 private key, write it to .appkeys.json,
and print the public key (hex) and FID for use with POST /v1/instances.

NOTE: This command is only available in dev builds (go build -tags dev).
In production TEE environments, the private key is provided by the hardware.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return initKeys(output)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", ".appkeys.json", "Output path for appkeys file")

	return cmd
}

func initKeys(output string) error {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return fmt.Errorf("generate random key: %w", err)
	}

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return fmt.Errorf("derive public key: %w", err)
	}

	privHex := hex.EncodeToString(priv[:])
	pubHex := hex.EncodeToString(pub)
	h := sha1.Sum(pub)
	fid := hex.EncodeToString(h[:])

	data, err := json.MarshalIndent(map[string]string{
		"env_crypt_key": privHex,
	}, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(output, data, 0600); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}

	fmt.Fprintf(os.Stderr, "Wrote %s\n", output)
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Public Key : %s\n", pubHex)
	fmt.Fprintf(os.Stderr, "FID        : %s\n", fid)
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Use the public key to register this instance:\n")
	fmt.Fprintf(os.Stderr, "  curl -X POST $SERVER/v1/instances \\\n")
	fmt.Fprintf(os.Stderr, "    -H 'Content-Type: application/json' \\\n")
	fmt.Fprintf(os.Stderr, "    -d '{\"public_key\":\"%s\",\"bound_app_id\":\"<APP_ID>\",\"bound_user_id\":\"<EMAIL>\"}'\n", pubHex)

	return nil
}
