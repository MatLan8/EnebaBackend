package models

import (
	"time"

	"github.com/google/uuid"
)

// CartItem mirrors a row in the cart_items table: one offer in a user's cart.
// An offer appears at most once per user; quantity is 1 to 10.
type CartItem struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"userId"`
	OfferID   uuid.UUID `json:"offerId"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"createdAt"`
}
