package requests

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"eneba-backend/internal/httpx"
)

// assertValidationFailed checks that err is a 400 validation_failed with a problem for each field.
func assertValidationFailed(t *testing.T, err error, fields ...string) {
	t.Helper()

	var apiErr *httpx.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v; want *httpx.APIError", err)
	}
	if apiErr.Status != http.StatusBadRequest || apiErr.Code != "validation_failed" {
		t.Errorf("status, code = %d, %q; want 400, validation_failed", apiErr.Status, apiErr.Code)
	}
	for _, field := range fields {
		if _, ok := apiErr.Details[field]; !ok {
			t.Errorf("details = %v; want a problem for %s", apiErr.Details, field)
		}
	}
	if len(apiErr.Details) != len(fields) {
		t.Errorf("details = %v; want problems only for %v", apiErr.Details, fields)
	}
}

func ptr[T any](v T) *T {
	return &v
}

func TestAddCartItemRequestValid(t *testing.T) {
	offerID := uuid.New()

	got, err := AddCartItemRequest{OfferID: ptr(offerID.String())}.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != offerID {
		t.Errorf("offerID = %s; want %s", got, offerID)
	}
}

func TestAddCartItemRequestInvalid(t *testing.T) {
	tests := []struct {
		name string
		req  AddCartItemRequest
	}{
		{name: "missing offer", req: AddCartItemRequest{}},
		{name: "empty offer", req: AddCartItemRequest{OfferID: ptr("")}},
		{name: "bad uuid", req: AddCartItemRequest{OfferID: ptr("abc")}},
		{name: "nil uuid", req: AddCartItemRequest{OfferID: ptr(uuid.Nil.String())}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.req.Validate()
			assertValidationFailed(t, err, "offerId")
		})
	}
}

func TestUpdateCartItemRequest(t *testing.T) {
	for _, quantity := range []int{1, 10} {
		got, err := UpdateCartItemRequest{Quantity: ptr(quantity)}.Validate()
		if err != nil || got != quantity {
			t.Errorf("Validate(%d) = %d, %v; want %d, nil", quantity, got, err, quantity)
		}
	}

	for _, req := range []UpdateCartItemRequest{{}, {Quantity: ptr(0)}, {Quantity: ptr(11)}} {
		_, err := req.Validate()
		assertValidationFailed(t, err, "quantity")
	}
}

func TestParseOfferIDParam(t *testing.T) {
	offerID := uuid.New()

	got, err := ParseOfferIDParam(offerID.String())
	if err != nil || got != offerID {
		t.Errorf("ParseOfferIDParam(valid) = %s, %v; want %s, nil", got, err, offerID)
	}

	for _, raw := range []string{"", "not-a-uuid", uuid.Nil.String()} {
		_, err := ParseOfferIDParam(raw)
		assertValidationFailed(t, err, "offerId")
	}
}
