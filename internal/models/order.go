package models

import (
	"time"

	"github.com/google/uuid"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"   // created at checkout, waiting for the payment
	OrderStatusPaid      OrderStatus = "paid"      // Stripe confirmed the payment, keys are sold
	OrderStatusCancelled OrderStatus = "cancelled" // the payment never completed, keys are released
)

// PaidAt is set exactly when Status is paid.
type Order struct {
	ID              uuid.UUID   `json:"id"`
	PublicID        string      `json:"publicId"` // the order number shown to the buyer
	UserID          uuid.UUID   `json:"userId"`
	Status          OrderStatus `json:"status"`
	TotalCents      int         `json:"totalCents"`
	Currency        string      `json:"currency"`
	StripeSessionID *string     `json:"-"` // nil until the Stripe Checkout session exists
	CreatedAt       time.Time   `json:"createdAt"`
	PaidAt          *time.Time  `json:"paidAt"`
	CartID          *uuid.UUID  `json:"-"` // the checked-out cart, emptied once paid; its ID is the cart cookie, so it is never sent
}
