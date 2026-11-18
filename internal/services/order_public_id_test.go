package services

import (
	"regexp"
	"testing"
)

func TestNewOrderPublicID(t *testing.T) {
	format := regexp.MustCompile(`^o-[a-z0-9]{7}$`)
	seen := make(map[string]bool)

	for range 1000 {
		id, err := NewOrderPublicID()
		if err != nil {
			t.Fatalf("NewOrderPublicID: %v", err)
		}
		if !format.MatchString(id) {
			t.Fatalf("id %q doesn't match o-[a-z0-9]{7}", id)
		}
		if seen[id] {
			t.Fatalf("id %q generated twice in 1000 tries", id)
		}
		seen[id] = true
	}
}
