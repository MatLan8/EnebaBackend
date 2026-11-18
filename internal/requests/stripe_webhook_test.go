package requests

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseStripeCheckoutSession(t *testing.T) {
	orderID := uuid.New()
	raw := []byte(`{"id":"cs_test_1","object":"checkout.session","payment_status":"paid","client_reference_id":"` +
		orderID.String() + `","metadata":{"order_id":"` + orderID.String() + `"},"amount_total":9278}`)

	session, err := ParseStripeCheckoutSession(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.ID != "cs_test_1" || session.PaymentStatus != "paid" {
		t.Errorf("session = %+v; want cs_test_1, paid", session)
	}
	if got, ok := session.OrderID(); !ok || got != orderID {
		t.Errorf("OrderID = %s, %v; want %s, true", got, ok, orderID)
	}

	if _, err := ParseStripeCheckoutSession([]byte(`{"payment_status":"paid"}`)); err == nil {
		t.Error("missing id: want an error")
	}
	if _, err := ParseStripeCheckoutSession([]byte(`not json`)); err == nil {
		t.Error("bad json: want an error")
	}
}

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
