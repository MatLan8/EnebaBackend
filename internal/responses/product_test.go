package responses

import "testing"

func TestDiscountPercent(t *testing.T) {
	intPtr := func(v int) *int {
		return &v
	}

	tests := []struct {
		name          string
		priceCents    int
		originalCents *int
		want          *int
	}{
		{"no original price", 1999, nil, nil},
		{"rounds down", 3299, intPtr(5999), intPtr(45)}, // 45.01%
		{"rounds up", 1000, intPtr(1999), intPtr(50)},   // 49.97%
		{"half", 50, intPtr(100), intPtr(50)},
		{"tiny discount", 999, intPtr(1000), intPtr(0)}, // 0.1%
		{"almost free", 1, intPtr(10000), intPtr(100)},  // 99.99%
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := discountPercent(tt.priceCents, tt.originalCents)

			switch {
			case tt.want == nil && got != nil:
				t.Errorf("discountPercent = %d; want nil", *got)
			case tt.want != nil && got == nil:
				t.Errorf("discountPercent = nil; want %d", *tt.want)
			case tt.want != nil && *got != *tt.want:
				t.Errorf("discountPercent = %d; want %d", *got, *tt.want)
			}
		})
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
