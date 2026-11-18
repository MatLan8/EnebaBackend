package models

import (
	"time"

	"github.com/google/uuid"
)

// DefaultCurrency is the currency of every offer today, the same as the default
// of offers.currency. It is used where there is no offer to read it from, e.g. an empty cart.
const DefaultCurrency = "EUR"

// Offer mirrors a row in the offers table: one seller's price for one product.
// Prices are whole cents, so 1999 means 19.99 EUR. Stock is not stored here,
// it is the number of unsold digital keys for the offer. Discounts are not
// stored either: they compare PriceCents with the product's BasePriceCents.
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
