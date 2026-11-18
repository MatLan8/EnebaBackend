package models

import (
	"time"

	"github.com/google/uuid"
)

// OrderItem mirrors a row in the order_items table: one digital key bought in
// an order. UnitPriceCents is the offer's price at checkout, so later price
// changes don't alter past orders.
type OrderItem struct {
	ID             uuid.UUID `json:"id"`
	OrderID        uuid.UUID `json:"orderId"`
	OfferID        uuid.UUID `json:"offerId"`
	DigitalKeyID   uuid.UUID `json:"digitalKeyId"`
	UnitPriceCents int       `json:"unitPriceCents"`
	CreatedAt      time.Time `json:"createdAt"`
}
