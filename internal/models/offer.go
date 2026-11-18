package models

import (
	"time"

	"github.com/google/uuid"
)

// Every offer is in EUR today; used where there is no offer to read the currency from, e.g. an empty cart.
const DefaultCurrency = "EUR"

// Prices are whole cents. Stock is not stored: it is the count of available digital keys.
// Discounts aren't stored either; they compare PriceCents with the product's BasePriceCents.
type Offer struct {
	ID         uuid.UUID `json:"id"`
	ProductID  uuid.UUID `json:"productId"`
	SellerID   uuid.UUID `json:"sellerId"`
	PriceCents int       `json:"priceCents"`
	Currency   string    `json:"currency"`
	IsActive   bool      `json:"isActive"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
