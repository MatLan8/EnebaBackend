package models

import (
	"time"

	"github.com/google/uuid"
)

// The key code is stored encrypted and only decrypted when shown to the buyer.
type DigitalKey struct {
	ID            uuid.UUID  `json:"id"`
	OfferID       uuid.UUID  `json:"offerId"`
	CodeEncrypted []byte     `json:"-"`
	CodeHash      string     `json:"-"`
	SoldAt        *time.Time `json:"soldAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	ReservedUntil *time.Time `json:"-"` // held for a checkout in progress until then
}

func (k *DigitalKey) IsSold() bool {
	return k.SoldAt != nil
}

// IsAvailable is the Go version of the repository's availableKeyCondition.
func (k *DigitalKey) IsAvailable(now time.Time) bool {
	return k.SoldAt == nil && (k.ReservedUntil == nil || k.ReservedUntil.Before(now))
}
