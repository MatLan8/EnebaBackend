package services

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func newTestKeyCipher(t *testing.T) *KeyCipher {
	t.Helper()

	secret := make([]byte, 32)
	rand.Read(secret)

	c, err := NewKeyCipher(secret)
	if err != nil {
		t.Fatalf("NewKeyCipher: %v", err)
	}
	return c
}

func TestKeyCipher_RoundTrip(t *testing.T) {
	c := newTestKeyCipher(t)
	code := "ABCDE-FGHIJ-KLMNO"

	got, err := c.Decrypt(c.Encrypt(code))
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != code {
		t.Fatalf("got %q, want %q", got, code)
	}
}

func TestKeyCipher_SameKeyEncryptsDifferently(t *testing.T) {
	c := newTestKeyCipher(t)

	a := c.Encrypt("ABCDE-FGHIJ-KLMNO")
	b := c.Encrypt("ABCDE-FGHIJ-KLMNO")
	if bytes.Equal(a, b) {
		t.Fatal("expected different ciphertexts for the same key")
	}
}

func TestKeyCipher_RejectsTamperedData(t *testing.T) {
	c := newTestKeyCipher(t)

	encrypted := c.Encrypt("ABCDE-FGHIJ-KLMNO")
	encrypted[len(encrypted)-1] ^= 0xFF // flip the bits of the last byte

	if _, err := c.Decrypt(encrypted); err == nil {
		t.Fatal("expected an error for tampered data")
	}
}

func TestKeyCipher_RejectsWrongSecret(t *testing.T) {
	encrypted := newTestKeyCipher(t).Encrypt("ABCDE-FGHIJ-KLMNO")

	if _, err := newTestKeyCipher(t).Decrypt(encrypted); err == nil {
		t.Fatal("expected an error when decrypting with another secret")
	}
}

func TestNewKeyCipher_RejectsShortSecret(t *testing.T) {
	if _, err := NewKeyCipher([]byte("too-short")); err == nil {
		t.Fatal("expected an error for a secret that isn't 32 bytes")
	}
}

func TestHashKeyCode_IsStable(t *testing.T) {
	if HashKeyCode("ABCDE-FGHIJ") != HashKeyCode("  ABCDE-FGHIJ ") {
		t.Fatal("expected surrounding spaces to be ignored")
	}
	if HashKeyCode("ABCDE-FGHIJ") == HashKeyCode("ABCDE-FGHIK") {
		t.Fatal("expected different keys to have different hashes")
	}
}
