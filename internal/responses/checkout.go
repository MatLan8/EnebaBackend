package responses

import "github.com/google/uuid"

// CheckoutResponse is returned by POST /api/v1/checkout. The frontend sends the
// buyer to CheckoutURL, Stripe's payment page. After paying, Stripe sends them
// back to /orders/{orderId}.
type CheckoutResponse struct {
	OrderID     uuid.UUID `json:"orderId"`
	CheckoutURL string    `json:"checkoutUrl"`
}
