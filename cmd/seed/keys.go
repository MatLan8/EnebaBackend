package main

import (
	"crypto/rand"
	"strings"
)

// keyAlphabet is A-Z and 2-9 without the look-alikes I, O, 0 and 1, like real
// store keys. It has exactly 32 characters and 256 is a multiple of 32, so
// "random byte % 32" picks every character with the same chance.
const keyAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// generateKeyCode returns a random fake key in the format "XXXXX-XXXXX-XXXXX".
func generateKeyCode() string {
	random := make([]byte, 15)
	rand.Read(random) // crypto/rand.Read never fails since Go 1.24

	var code strings.Builder
	for i, b := range random {
		if i > 0 && i%5 == 0 {
			code.WriteByte('-')
		}
		code.WriteByte(keyAlphabet[int(b)%len(keyAlphabet)])
	}
	return code.String()
}
