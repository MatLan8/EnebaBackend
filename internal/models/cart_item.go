package models

import (
	"time"

	"github.com/google/uuid"
)

// MaxCartItemQuantity is the most keys of one offer a cart can hold. It must match
// the CHECK on cart_items.quantity in migrations/000007_cart_items.up.sql.
const MaxCartItemQuantity = 10

// CartItem mirrors a row in the cart_items table: one offer in a cart.
// An offer appears at most once per cart; quantity is 1 to 10.
type CartItem struct {
	ID        uuid.UUID `json:"id"`
	CartID    uuid.UUID `json:"cartId"`
	OfferID   uuid.UUID `json:"offerId"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"createdAt"`
}
