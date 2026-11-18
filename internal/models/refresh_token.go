package models

import (
	"time"

	"github.com/google/uuid"
)

// Only the SHA-256 hash is stored; the raw token lives in the user's cookie.
type RefreshToken struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	TokenHash           string
	ExpiresAt           time.Time
	RevokedAt           *time.Time // nil while the token is active
	ReplacedByTokenHash *string    // nil until the token is rotated
	CreatedAt           time.Time
}

func (t *RefreshToken) IsActive(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}

// Rotating sets RevokedAt, so it is the rotation time of a replaced token.
func (t *RefreshToken) WasRotatedWithin(window time.Duration, now time.Time) bool {
	return t.ReplacedByTokenHash != nil && t.RevokedAt != nil && now.Sub(*t.RevokedAt) < window
}
