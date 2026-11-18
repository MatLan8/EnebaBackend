package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestProductDetailRowToResponse(t *testing.T) {
	row := productDetailRow{
		ID:             uuid.New(),
		ReleaseDate:    time.Date(2022, time.February, 25, 0, 0, 0, 0, time.UTC),
		BasePriceCents: 5999,
	}
	offers := []offerRow{
		{ID: uuid.New(), PriceCents: 4559, Currency: "EUR", SellerName: "PixelVault"},
		{ID: uuid.New(), PriceCents: 5999, Currency: "EUR", SellerName: "KeyHub"},
	}

	got := row.toResponse(nil, offers)

	if got.ReleaseDate != "2022-02-25" {
		t.Errorf("releaseDate = %q; want %q", got.ReleaseDate, "2022-02-25")
	}
	if len(got.Offers) != 2 || got.Offers[0].ID != offers[0].ID || got.Offers[1].ID != offers[1].ID {
		t.Fatalf("offers not kept in query order: %+v", got.Offers)
	}
	if d := got.Offers[0].Price.DiscountPercent; d == nil || *d != 24 {
		t.Errorf("discounted offer discountPercent = %v; want 24", d)
	}
	if d := got.Offers[1].Price.DiscountPercent; d != nil {
		t.Errorf("offer at base price discountPercent = %d; want nil", *d)
	}
	if got.Offers[0].Seller.Name != "PixelVault" {
		t.Errorf("seller name = %q; want %q", got.Offers[0].Seller.Name, "PixelVault")
	}
}

func TestProductDetailRowToResponseWithoutOffers(t *testing.T) {
	got := productDetailRow{}.toResponse(nil, nil)

	// A nil slice would be null in the JSON; the product page expects [].
	if got.Offers == nil {
		t.Error("offers = nil; want an empty slice")
	}
}
