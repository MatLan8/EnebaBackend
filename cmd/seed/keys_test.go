package main

import (
	"regexp"
	"testing"
)

func TestGenerateKeyCode_Format(t *testing.T) {
	format := regexp.MustCompile(`^[A-HJ-NP-Z2-9]{5}-[A-HJ-NP-Z2-9]{5}-[A-HJ-NP-Z2-9]{5}$`)

	for range 100 {
		code := generateKeyCode()
		if !format.MatchString(code) {
			t.Fatalf("code %q does not match XXXXX-XXXXX-XXXXX", code)
		}
	}
}

func TestGenerateKeyCode_Random(t *testing.T) {
	seen := make(map[string]bool)

	for range 1000 {
		code := generateKeyCode()
		if seen[code] {
			t.Fatalf("code %q generated twice", code)
		}
		seen[code] = true
	}
}

func TestKeyAlphabet_Has32Characters(t *testing.T) {
	if len(keyAlphabet) != 32 {
		t.Fatalf("keyAlphabet has %d characters, want 32 so byte %% 32 stays unbiased", len(keyAlphabet))
	}
}
