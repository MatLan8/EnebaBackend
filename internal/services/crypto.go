package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// KeyCipher encrypts and decrypts digital game keys with AES-256-GCM.
// GCM both hides the key and detects tampering: if a single byte of the
// stored value changes, Decrypt fails instead of returning garbage.
type KeyCipher struct {
	aead cipher.AEAD
}

// NewKeyCipher creates a KeyCipher from a 32-byte secret (AES-256).
func NewKeyCipher(secret []byte) (*KeyCipher, error) {
	if len(secret) != 32 {
		return nil, fmt.Errorf("key encryption secret must be 32 bytes, got %d", len(secret))
	}

	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	// NewGCMWithRandomNonce generates a fresh random nonce on every Seal and
	// stores it in front of the ciphertext, so we don't have to manage nonces.
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	return &KeyCipher{aead: aead}, nil
}

// Encrypt returns the encrypted form of a plain key, ready for the code_encrypted column.
// Encrypting the same key twice gives different bytes, because the nonce is random.
func (c *KeyCipher) Encrypt(code string) []byte {
	return c.aead.Seal(nil, nil, []byte(code), nil)
}

// Decrypt returns the plain key. It fails if the data was changed or was
// encrypted with a different secret.
func (c *KeyCipher) Decrypt(encrypted []byte) (string, error) {
	plain, err := c.aead.Open(nil, nil, encrypted, nil)
	if err != nil {
		return "", errors.New("decrypt digital key: invalid data or wrong secret")
	}

	return string(plain), nil
}

// HashKeyCode returns the SHA-256 hex digest of a plain key for the code_hash column.
// Encrypted values are different every time, so the database can't spot duplicates
// from them; the hash is always the same for the same key, so a UNIQUE index can.
func HashKeyCode(code string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}
