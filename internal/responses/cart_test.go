package responses

import "testing"

func TestNewCartResponseTotals(t *testing.T) {
	items := []CartItemResponse{
		{Quantity: 1, PriceCents: 4559},
		{Quantity: 4, PriceCents: 299},
		{Quantity: 2, PriceCents: 1050},
	}

	got := NewCartResponse(items, "EUR")

	// itemCount counts keys, not rows: 1 + 4 + 2.
	if got.ItemCount != 7 {
		t.Errorf("itemCount = %d; want 7", got.ItemCount)
	}
	// 4559 + 4*299 + 2*1050
	if got.TotalCents != 7855 {
		t.Errorf("totalCents = %d; want 7855", got.TotalCents)
	}
	if len(got.Items) != 3 {
		t.Errorf("items = %d; want 3", len(got.Items))
	}
}
