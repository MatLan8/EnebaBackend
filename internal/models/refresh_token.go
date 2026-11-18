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
