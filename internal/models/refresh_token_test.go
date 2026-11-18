package models

import (
	"testing"
	"time"
)

func TestRefreshTokenWasRotatedWithin(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	successor := "successor-hash"
	at := func(ago time.Duration) *time.Time {
		v := now.Add(-ago)
		return &v
	}

	tests := []struct {
		name  string
		token RefreshToken
		want  bool
	}{
		{"rotated just now", RefreshToken{RevokedAt: at(time.Second), ReplacedByTokenHash: &successor}, true},
		{"rotated before the window", RefreshToken{RevokedAt: at(time.Minute), ReplacedByTokenHash: &successor}, false},
		{"revoked by logout, not rotated", RefreshToken{RevokedAt: at(time.Second)}, false},
		{"active", RefreshToken{}, false},
	}

	for _, tt := range tests {
		if got := tt.token.WasRotatedWithin(30*time.Second, now); got != tt.want {
			t.Errorf("%s: WasRotatedWithin = %v; want %v", tt.name, got, tt.want)
		}
	}
}
