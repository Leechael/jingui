package client

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

type appKeysFile struct {
	EnvCryptKey string `json:"env_crypt_key"`
}

// LoadPrivateKey reads .appkeys.json and returns the X25519 private key.
func LoadPrivateKey(path string) ([32]byte, error) {
	var key [32]byte

	data, err := os.ReadFile(path)
	if err != nil {
		return key, fmt.Errorf("read appkeys file: %w", err)
	}

	var akf appKeysFile
	if err := json.Unmarshal(data, &akf); err != nil {
		return key, fmt.Errorf("parse appkeys file: %w", err)
	}

	if akf.EnvCryptKey == "" {
		return key, fmt.Errorf("env_crypt_key is empty in appkeys file")
	}

	decoded, err := hex.DecodeString(akf.EnvCryptKey)
	if err != nil {
		return key, fmt.Errorf("decode env_crypt_key hex: %w", err)
	}

	if len(decoded) != 32 {
		return key, fmt.Errorf("env_crypt_key must be 32 bytes, got %d", len(decoded))
	}

	copy(key[:], decoded)
	return key, nil
}
