package models

import (
	"time"

	"github.com/google/uuid"
)

// Cart mirrors a row in the carts table. A cart belongs to a browser, not to a
// user: its ID is kept in the browser's cart_token cookie, so anyone can fill a
// cart before signing in. Its items are rows in cart_items.
type Cart struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"` // last time an item was added, changed or removed
}
