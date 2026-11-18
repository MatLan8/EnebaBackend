package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/repository"
)

func TestCheckoutLineItems(t *testing.T) {
	eldenRing := uuid.New()
	sims := uuid.New()
	lines := []repository.CheckoutLine{
		{OfferID: eldenRing, Quantity: 2, PriceCents: 4639, Title: "Elden Ring", ImageURL: "https://img/er.jpg"},
		{OfferID: sims, Quantity: 1, PriceCents: 2039, Title: "The Sims 4 DLC"},
		{OfferID: eldenRing, Quantity: 1, PriceCents: 4639, Title: "Elden Ring"},
	}

	got := checkoutLineItems(lines)

	if len(got) != 2 {
		t.Fatalf("got %d line items; want 2 (one per offer): %+v", len(got), got)
	}
	if got[0].Name != "Elden Ring" || got[0].Quantity != 3 || got[0].UnitAmountCents != 4639 || got[0].ImageURL != "https://img/er.jpg" {
		t.Errorf("first item = %+v; want Elden Ring x3 at 4639 with its image", got[0])
	}
	if got[1].Name != "The Sims 4 DLC" || got[1].Quantity != 1 || got[1].UnitAmountCents != 2039 {
		t.Errorf("second item = %+v; want The Sims 4 DLC x1 at 2039", got[1])
	}
}

func TestSummarizeCheckout(t *testing.T) {
	lines := []repository.CheckoutLine{
		{OfferID: uuid.New(), Quantity: 2, PriceCents: 4639, Currency: "EUR", IsActive: true},
		{OfferID: uuid.New(), Quantity: 1, PriceCents: 2039, Currency: "EUR", IsActive: true},
	}

	total, currency, err := summarizeCheckout(lines)
	if err != nil || total != 2*4639+2039 || currency != "EUR" {
		t.Errorf("summarizeCheckout = %d, %q, %v; want %d, EUR, nil", total, currency, err, 2*4639+2039)
	}
}

func TestSummarizeCheckoutErrors(t *testing.T) {
	inactive := uuid.New()

	assertAPIError := func(t *testing.T, err error, status int, code string) *httpx.APIError {
		t.Helper()
		var apiErr *httpx.APIError
		if !errors.As(err, &apiErr) || apiErr.Status != status || apiErr.Code != code {
			t.Fatalf("err = %v; want %d %s", err, status, code)
		}
		return apiErr
	}

	_, _, err := summarizeCheckout(nil)
	assertAPIError(t, err, http.StatusBadRequest, "cart_empty")

	_, _, err = summarizeCheckout([]repository.CheckoutLine{
		{OfferID: uuid.New(), Quantity: 1, PriceCents: 100, Currency: "EUR", IsActive: true},
		{OfferID: inactive, Quantity: 1, PriceCents: 100, Currency: "EUR", IsActive: false},
	})
	apiErr := assertAPIError(t, err, http.StatusConflict, "offer_unavailable")
	if apiErr.Details["offerId"] != inactive.String() {
		t.Errorf("details = %v; want offerId %s", apiErr.Details, inactive)
	}

	_, _, err = summarizeCheckout([]repository.CheckoutLine{
		{OfferID: uuid.New(), Quantity: 1, PriceCents: 100, Currency: "EUR", IsActive: true},
		{OfferID: uuid.New(), Quantity: 1, PriceCents: 100, Currency: "USD", IsActive: true},
	})
	var apiErrMixed *httpx.APIError
	if err == nil || errors.As(err, &apiErrMixed) {
		t.Errorf("mixed currencies: err = %v; want a plain (500) error", err)
	}
}
