package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

const (
	pubKeyLen  = 32
	ivLen      = 12
	gcmTagLen  = 16
	minBlobLen = pubKeyLen + ivLen + gcmTagLen // 60 bytes minimum (empty plaintext)
)

// Encrypt performs ECIES encryption using X25519 + AES-256-GCM.
// Output format: ephemeralPubKey(32) || iv(12) || ciphertext+tag
func Encrypt(recipientPubKey [32]byte, plaintext []byte) ([]byte, error) {
	// Generate ephemeral X25519 keypair
	var ephPriv [32]byte
	if _, err := rand.Read(ephPriv[:]); err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}
	ephPub, err := curve25519.X25519(ephPriv[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("derive ephemeral public key: %w", err)
	}

	// DH shared secret
	shared, err := curve25519.X25519(ephPriv[:], recipientPubKey[:])
	if err != nil {
		return nil, fmt.Errorf("compute shared secret: %w", err)
	}

	// AES-256-GCM encrypt
	block, err := aes.NewCipher(shared)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	iv := make([]byte, ivLen)
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("generate IV: %w", err)
	}

	ct := gcm.Seal(nil, iv, plaintext, nil)

	// ephPK(32) || iv(12) || ct+tag
	out := make([]byte, 0, pubKeyLen+ivLen+len(ct))
	out = append(out, ephPub...)
	out = append(out, iv...)
	out = append(out, ct...)
	return out, nil
}

// Decrypt performs ECIES decryption using X25519 + AES-256-GCM.
// Input format: ephemeralPubKey(32) || iv(12) || ciphertext+tag
func Decrypt(privateKey [32]byte, data []byte) ([]byte, error) {
	if len(data) < minBlobLen {
		return nil, errors.New("ciphertext too short")
	}

	ephPub := data[:pubKeyLen]
	iv := data[pubKeyLen : pubKeyLen+ivLen]
	ct := data[pubKeyLen+ivLen:]

	// DH shared secret
	shared, err := curve25519.X25519(privateKey[:], ephPub)
	if err != nil {
		return nil, fmt.Errorf("compute shared secret: %w", err)
	}

	// AES-256-GCM decrypt
	block, err := aes.NewCipher(shared)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, iv, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}
