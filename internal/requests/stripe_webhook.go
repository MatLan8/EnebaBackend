package requests

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// StripeCheckoutSession holds the few fields of a Stripe Checkout session that
// the webhook needs. Everything else in the event is ignored.
type StripeCheckoutSession struct {
	ID                string            `json:"id"`             // cs_test_..., saved on the order at checkout
	PaymentStatus     string            `json:"payment_status"` // "paid" once the money is captured
	ClientReferenceID string            `json:"client_reference_id"`
	Metadata          map[string]string `json:"metadata"`
}

// ParseStripeCheckoutSession reads the session from a webhook event's data.object.
func ParseStripeCheckoutSession(object json.RawMessage) (StripeCheckoutSession, error) {
	var session StripeCheckoutSession
	if err := json.Unmarshal(object, &session); err != nil {
		return StripeCheckoutSession{}, fmt.Errorf("parse checkout session: %w", err)
	}
	if session.ID == "" {
		return StripeCheckoutSession{}, fmt.Errorf("parse checkout session: id is missing")
	}

	return session, nil
}

// OrderID returns the order ID checkout put in the session's metadata (or in
// client_reference_id). ok is false for sessions not created by our checkout,
// e.g. the ones `stripe trigger` makes.
func (s StripeCheckoutSession) OrderID() (uuid.UUID, bool) {
	for _, raw := range []string{s.Metadata["order_id"], s.ClientReferenceID} {
		if id, err := uuid.Parse(raw); err == nil && id != uuid.Nil {
			return id, true
		}
	}

	return uuid.Nil, false
}
