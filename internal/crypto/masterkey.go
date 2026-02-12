package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

const minAtRestLen = ivLen + gcmTagLen // 28 bytes minimum

// EncryptAtRest encrypts plaintext using AES-256-GCM with the given master key.
// Output format: iv(12) || ciphertext+tag
func EncryptAtRest(masterKey [32]byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey[:])
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

	out := make([]byte, 0, ivLen+len(ct))
	out = append(out, iv...)
	out = append(out, ct...)
	return out, nil
}

// DecryptAtRest decrypts data encrypted with EncryptAtRest.
// Input format: iv(12) || ciphertext+tag
func DecryptAtRest(masterKey [32]byte, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < minAtRestLen {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertext[:ivLen]
	ct := ciphertext[ivLen:]

	block, err := aes.NewCipher(masterKey[:])
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
