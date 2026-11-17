package models

import (
	"time"

	"github.com/google/uuid"
)

// DigitalKey mirrors a row in the digital_keys table.
// The key itself is stored encrypted and is never serialized as-is;
// it is decrypted only when showing it to the buyer.
type DigitalKey struct {
	ID            uuid.UUID  `json:"id"`
	OfferID       uuid.UUID  `json:"offerId"`
	CodeEncrypted []byte     `json:"-"`
	CodeHash      string     `json:"-"`
	OrderItemID   *uuid.UUID `json:"orderItemId"` // nil while unsold
	SoldAt        *time.Time `json:"soldAt"`
	CreatedAt     time.Time  `json:"createdAt"`
}

// IsSold reports whether the key has been bought.
func (k *DigitalKey) IsSold() bool {
	return k.OrderItemID != nil
}
