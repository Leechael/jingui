package crypto

import (
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/curve25519"
)

func generateKeypair(t *testing.T) ([32]byte, [32]byte) {
	t.Helper()
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		t.Fatal(err)
	}
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	var pubKey [32]byte
	copy(pubKey[:], pub)
	return priv, pubKey
}

func TestECIES_RoundTrip(t *testing.T) {
	priv, pub := generateKeypair(t)

	plaintext := []byte("hello, jingui!")
	blob, err := Encrypt(pub, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := Decrypt(priv, blob)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if string(got) != string(plaintext) {
		t.Fatalf("got %q, want %q", got, plaintext)
	}
}

func TestECIES_EmptyPlaintext(t *testing.T) {
	priv, pub := generateKeypair(t)

	blob, err := Encrypt(pub, []byte{})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := Decrypt(priv, blob)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("expected empty plaintext, got %d bytes", len(got))
	}
}

func TestECIES_WrongKey(t *testing.T) {
	_, pub := generateKeypair(t)
	otherPriv, _ := generateKeypair(t)

	blob, err := Encrypt(pub, []byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = Decrypt(otherPriv, blob)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestECIES_ShortData(t *testing.T) {
	var priv [32]byte
	_, err := Decrypt(priv, []byte("too short"))
	if err == nil {
		t.Fatal("expected error for short data")
	}
}
