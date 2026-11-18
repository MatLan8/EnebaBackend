package repository

import (
	"testing"

	"github.com/google/uuid"

	"eneba-backend/internal/models"
)

func TestNewCartResponse(t *testing.T) {
	logo := "https://example.com/neonkey.png"
	rows := []cartItemRow{
		{OfferID: uuid.New(), Quantity: 2, AvailableQuantity: 5, PriceCents: 4559, Currency: "EUR", Type: models.ProductTypeGame, SellerName: "NeonKey", SellerLogoURL: &logo},
		{OfferID: uuid.New(), Quantity: 1, AvailableQuantity: 3, PriceCents: 999, Currency: "EUR", Type: models.ProductTypeDLC, SellerName: "KeyHub"},
	}

	got := newCartResponse(rows)

	if len(got.Items) != 2 || got.Items[0].OfferID != rows[0].OfferID || got.Items[1].OfferID != rows[1].OfferID {
		t.Fatalf("items not kept in query order: %+v", got.Items)
	}
	if got.Items[0].Product.IsDLC || !got.Items[1].Product.IsDLC {
		t.Errorf("isDlc = %v, %v; want false, true", got.Items[0].Product.IsDLC, got.Items[1].Product.IsDLC)
	}
	if got.Items[0].Seller.LogoURL == nil || *got.Items[0].Seller.LogoURL != logo {
		t.Errorf("seller logoUrl = %v; want %q", got.Items[0].Seller.LogoURL, logo)
	}
	if got.Items[1].Seller.LogoURL != nil {
		t.Errorf("seller without a logo has logoUrl %q; want nil", *got.Items[1].Seller.LogoURL)
	}
	if got.Items[0].AvailableQuantity != 5 {
		t.Errorf("availableQuantity = %d; want 5", got.Items[0].AvailableQuantity)
	}
	if got.ItemCount != 3 || got.TotalCents != 2*4559+999 || got.Currency != "EUR" {
		t.Errorf("itemCount, totalCents, currency = %d, %d, %q; want 3, %d, EUR", got.ItemCount, got.TotalCents, got.Currency, 2*4559+999)
	}
}

func TestNewCartResponseNoRows(t *testing.T) {
	got := newCartResponse(nil)

	if got.Items == nil || len(got.Items) != 0 {
		t.Errorf("items = %#v; want an empty, non-nil slice", got.Items)
	}
	if got.Currency != models.DefaultCurrency {
		t.Errorf("currency = %q; want %q", got.Currency, models.DefaultCurrency)
	}
}
