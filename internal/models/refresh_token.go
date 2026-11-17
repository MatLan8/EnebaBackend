package models

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken mirrors a row in the refresh_tokens table.
// Only the SHA-256 hash of the token is stored; the raw value lives in the user's cookie.
type RefreshToken struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	TokenHash           string
	ExpiresAt           time.Time
	RevokedAt           *time.Time // nil while the token is active
	ReplacedByTokenHash *string    // nil until the token is rotated
	CreatedAt           time.Time
}

// IsActive reports whether the token can still be used.
func (t *RefreshToken) IsActive(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}
