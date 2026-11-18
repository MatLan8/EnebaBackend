package requests

import (
	"testing"

	"github.com/google/uuid"
)

func TestStripeCheckoutSessionOrderID(t *testing.T) {
	orderID := uuid.New()

	fromReference := StripeCheckoutSession{ClientReferenceID: orderID.String()}
	if got, ok := fromReference.OrderID(); !ok || got != orderID {
		t.Errorf("client_reference_id only: OrderID = %s, %v; want %s, true", got, ok, orderID)
	}

	foreign := StripeCheckoutSession{ID: "cs_test_triggered", Metadata: map[string]string{"order_id": "x"}}
	if _, ok := foreign.OrderID(); ok {
		t.Error("session without our order id: ok = true; want false")
	}
}
