package models

import (
	"time"

	"github.com/google/uuid"
)

// A cart belongs to a browser (cart_token cookie), not a user, so it can be filled before signing in.
type Cart struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"` // last time an item was added, changed or removed
}
