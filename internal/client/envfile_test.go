package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `# Comment
FOO=bar
BAZ="quoted value"
SINGLE='single quoted'
EMPTY=

# Another comment
REF=jingui://app/user/field
`
	os.WriteFile(path, []byte(content), 0600)

	entries, err := ParseEnvFile(path)
	if err != nil {
		t.Fatalf("ParseEnvFile: %v", err)
	}

	expected := []EnvEntry{
		{Key: "FOO", Value: "bar"},
		{Key: "BAZ", Value: "quoted value"},
		{Key: "SINGLE", Value: "single quoted"},
		{Key: "EMPTY", Value: ""},
		{Key: "REF", Value: "jingui://app/user/field"},
	}

	if len(entries) != len(expected) {
		t.Fatalf("got %d entries, want %d", len(entries), len(expected))
	}

	for i, e := range entries {
		if e.Key != expected[i].Key || e.Value != expected[i].Value {
			t.Errorf("entry[%d] = {%q, %q}, want {%q, %q}", i, e.Key, e.Value, expected[i].Key, expected[i].Value)
		}
	}
}

func TestParseEnvFile_MissingEquals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("BADLINE\n"), 0600)

	_, err := ParseEnvFile(path)
	if err == nil {
		t.Fatal("expected error for missing '='")
	}
}
