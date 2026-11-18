package models

import (
	"time"

	"github.com/google/uuid"
)

// DigitalKey mirrors a row in the digital_keys table, in column order.
// The key itself is stored encrypted and is never serialized as-is;
// it is decrypted only when showing it to the buyer.
// The order that bought a key is found through order_items.digital_key_id.
type DigitalKey struct {
	ID            uuid.UUID  `json:"id"`
	OfferID       uuid.UUID  `json:"offerId"`
	CodeEncrypted []byte     `json:"-"`
	CodeHash      string     `json:"-"`
	SoldAt        *time.Time `json:"soldAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	ReservedUntil *time.Time `json:"-"` // held for a checkout in progress until then
}

// IsSold reports whether the key has been bought.
func (k *DigitalKey) IsSold() bool {
	return k.SoldAt != nil
}

// IsAvailable reports whether the key can be bought at time now: it is not sold
// and not reserved by a checkout, or that reservation has expired.
// It is the Go version of the repository's availableKeyCondition.
func (k *DigitalKey) IsAvailable(now time.Time) bool {
	return k.SoldAt == nil && (k.ReservedUntil == nil || k.ReservedUntil.Before(now))
}
