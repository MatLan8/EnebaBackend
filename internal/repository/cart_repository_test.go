package repository

import "testing"

func TestCartItemRowAvailability(t *testing.T) {
	tests := []struct {
		name          string
		row           cartItemRow
		wantAvailable bool
		wantQuantity  int
	}{
		{"enough keys", cartItemRow{Quantity: 2, AvailableQuantity: 2, IsActive: true}, true, 2},
		{"too few keys", cartItemRow{Quantity: 3, AvailableQuantity: 2, IsActive: true}, false, 2},
		{"inactive offer with keys", cartItemRow{Quantity: 1, AvailableQuantity: 5, IsActive: false}, false, 0},
	}

	for _, tt := range tests {
		got := tt.row.toResponse()
		if got.Available != tt.wantAvailable || got.AvailableQuantity != tt.wantQuantity {
			t.Errorf("%s: available, availableQuantity = %v, %d; want %v, %d",
				tt.name, got.Available, got.AvailableQuantity, tt.wantAvailable, tt.wantQuantity)
		}
	}
}
