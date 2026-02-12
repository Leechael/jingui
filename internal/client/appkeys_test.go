package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrivateKey_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".appkeys.json")
	// 32 bytes = 64 hex chars
	os.WriteFile(path, []byte(`{"env_crypt_key":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`), 0600)

	key, err := LoadPrivateKey(path)
	if err != nil {
		t.Fatalf("LoadPrivateKey: %v", err)
	}
	for _, b := range key {
		if b != 0xaa {
			t.Fatalf("expected all 0xaa bytes, got %02x", b)
		}
	}
}

func TestLoadPrivateKey_MissingFile(t *testing.T) {
	_, err := LoadPrivateKey("/nonexistent/.appkeys.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadPrivateKey_BadHex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".appkeys.json")
	os.WriteFile(path, []byte(`{"env_crypt_key":"not-hex"}`), 0600)

	_, err := LoadPrivateKey(path)
	if err == nil {
		t.Fatal("expected error for bad hex")
	}
}

func TestLoadPrivateKey_WrongLength(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".appkeys.json")
	os.WriteFile(path, []byte(`{"env_crypt_key":"aabb"}`), 0600)

	_, err := LoadPrivateKey(path)
	if err == nil {
		t.Fatal("expected error for wrong length")
	}
}
