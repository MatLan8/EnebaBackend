package httpx

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// The frontend reads availableQuantity as a number, so it must not be sent as a string.
func TestErrorDetailsKeepTheirJSONType(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, httptest.NewRequest("POST", "/api/v1/checkout", nil), InsufficientStockForOffer("offer-1", 3))

	var body struct {
		Error struct {
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got, ok := body.Error.Details["availableQuantity"].(float64); !ok || got != 3 {
		t.Errorf("availableQuantity = %#v; want the number 3", body.Error.Details["availableQuantity"])
	}
	if got := body.Error.Details["offerId"]; got != "offer-1" {
		t.Errorf("offerId = %#v; want offer-1", got)
	}
}
