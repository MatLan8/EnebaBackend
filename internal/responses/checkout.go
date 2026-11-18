package responses

import "github.com/google/uuid"

// After paying, Stripe sends the buyer back to /orders/{orderId}.
type CheckoutResponse struct {
	OrderID     uuid.UUID `json:"orderId"`
	CheckoutURL string    `json:"checkoutUrl"`
}
