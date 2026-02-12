package crypto

import (
	"crypto/rand"
	"testing"
)

func randomMasterKey(t *testing.T) [32]byte {
	t.Helper()
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestAtRest_RoundTrip(t *testing.T) {
	key := randomMasterKey(t)
	plaintext := []byte("sensitive data")

	ct, err := EncryptAtRest(key, plaintext)
	if err != nil {
		t.Fatalf("EncryptAtRest: %v", err)
	}

	got, err := DecryptAtRest(key, ct)
	if err != nil {
		t.Fatalf("DecryptAtRest: %v", err)
	}

	if string(got) != string(plaintext) {
		t.Fatalf("got %q, want %q", got, plaintext)
	}
}

func TestAtRest_WrongKey(t *testing.T) {
	key := randomMasterKey(t)
	wrongKey := randomMasterKey(t)

	ct, err := EncryptAtRest(key, []byte("secret"))
	if err != nil {
		t.Fatalf("EncryptAtRest: %v", err)
	}

	_, err = DecryptAtRest(wrongKey, ct)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestAtRest_ShortData(t *testing.T) {
	key := randomMasterKey(t)
	_, err := DecryptAtRest(key, []byte("short"))
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestAtRest_EmptyPlaintext(t *testing.T) {
	key := randomMasterKey(t)

	ct, err := EncryptAtRest(key, []byte{})
	if err != nil {
		t.Fatalf("EncryptAtRest: %v", err)
	}

	got, err := DecryptAtRest(key, ct)
	if err != nil {
		t.Fatalf("DecryptAtRest: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("expected empty plaintext, got %d bytes", len(got))
	}
}
