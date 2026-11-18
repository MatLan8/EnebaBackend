package handlers

import (
	"testing"

	"eneba-backend/internal/models"
)

func TestWebhookActionFor(t *testing.T) {
	tests := []struct {
		eventType     string
		paymentStatus string
		want          webhookAction
	}{
		{"checkout.session.completed", "paid", webhookMarkPaid},
		{"checkout.session.completed", "unpaid", webhookIgnore},
		{"checkout.session.expired", "unpaid", webhookCancel},
		{"payment_intent.succeeded", "", webhookIgnore},
		{"charge.refunded", "paid", webhookIgnore},
	}

	for _, tt := range tests {
		if got := webhookActionFor(tt.eventType, tt.paymentStatus); got != tt.want {
			t.Errorf("webhookActionFor(%q, %q) = %v; want %v", tt.eventType, tt.paymentStatus, got, tt.want)
		}
	}
}

func TestPaidOutcomeFor(t *testing.T) {
	tests := map[models.OrderStatus]paidOutcome{
		models.OrderStatusPending:   paidSellKeys,
		models.OrderStatusPaid:      paidAlreadyDone,
		models.OrderStatusCancelled: paidAfterCancelling,
	}

	for status, want := range tests {
		if got := paidOutcomeFor(status); got != want {
			t.Errorf("paidOutcomeFor(%q) = %v; want %v", status, got, want)
		}
	}
}
