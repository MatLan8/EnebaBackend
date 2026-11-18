package models

import (
	"time"

	"github.com/google/uuid"
)

// Must match the CHECK on cart_items.quantity in migrations/000007_cart_items.up.sql.
const MaxCartItemQuantity = 10

type CartItem struct {
	ID        uuid.UUID `json:"id"`
	CartID    uuid.UUID `json:"cartId"`
	OfferID   uuid.UUID `json:"offerId"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"createdAt"`
}
