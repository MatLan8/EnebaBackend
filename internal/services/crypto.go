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

// AES-256-GCM also detects tampering: one changed byte makes Decrypt fail instead of returning garbage.
type KeyCipher struct {
	aead cipher.AEAD
}

func NewKeyCipher(secret []byte) (*KeyCipher, error) {
	if len(secret) != 32 {
		return nil, fmt.Errorf("key encryption secret must be 32 bytes, got %d", len(secret))
	}

	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	// A fresh random nonce is stored in front of every ciphertext, so we don't manage nonces.
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	return &KeyCipher{aead: aead}, nil
}

// The nonce is random, so encrypting the same key twice gives different bytes.
func (c *KeyCipher) Encrypt(code string) []byte {
	return c.aead.Seal(nil, nil, []byte(code), nil)
}

func (c *KeyCipher) Decrypt(encrypted []byte) (string, error) {
	plain, err := c.aead.Open(nil, nil, encrypted, nil)
	if err != nil {
		return "", errors.New("decrypt digital key: invalid data or wrong secret")
	}

	return string(plain), nil
}

// Encrypted values differ every time, so duplicates are found by this hash (UNIQUE index) instead.
func HashKeyCode(code string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}
