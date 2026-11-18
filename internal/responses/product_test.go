package responses

import "testing"

func TestNewPriceResponse(t *testing.T) {
	intPtr := func(v int) *int {
		return &v
	}

	tests := []struct {
		name         string
		priceCents   int
		baseCents    int
		wantDiscount *int
	}{
		{"rounds down", 3299, 5999, intPtr(45)}, // 45.01%
		{"rounds up", 1000, 1999, intPtr(50)},   // 49.97%
		{"half", 50, 100, intPtr(50)},
		{"tiny discount", 999, 1000, intPtr(0)}, // 0.1%
		{"almost free", 1, 10000, intPtr(100)},  // 99.99%
		{"same as base price", 1999, 1999, nil},
		{"above base price", 2499, 1999, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPriceResponse(tt.priceCents, tt.baseCents, "EUR")

			if got.Cents != tt.priceCents || got.Currency != "EUR" {
				t.Errorf("cents, currency = %d, %q; want %d, %q", got.Cents, got.Currency, tt.priceCents, "EUR")
			}
			checkOptionalInt(t, "discountPercent", got.DiscountPercent, tt.wantDiscount)
		})
	}
}

// checkOptionalInt compares two nullable ints, where nil means null in the JSON.
func checkOptionalInt(t *testing.T, field string, got, want *int) {
	t.Helper()

	switch {
	case want == nil && got != nil:
		t.Errorf("%s = %d; want nil", field, *got)
	case want != nil && got == nil:
		t.Errorf("%s = nil; want %d", field, *want)
	case want != nil && *got != *want:
		t.Errorf("%s = %d; want %d", field, *got, *want)
	}
}

func TestTotalPages(t *testing.T) {
	tests := []struct {
		total int
		limit int
		want  int
	}{
		{0, 24, 0},
		{1, 24, 1},
		{24, 24, 1},
		{25, 24, 2},
		{52, 24, 3},
		{52, 60, 1},
	}

	for _, tt := range tests {
		if got := totalPages(tt.total, tt.limit); got != tt.want {
			t.Errorf("totalPages(%d, %d) = %d; want %d", tt.total, tt.limit, got, tt.want)
		}
	}
}
