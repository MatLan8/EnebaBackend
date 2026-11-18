package models

import (
	"testing"
	"time"
)

func TestDigitalKeyIsAvailable(t *testing.T) {
	now := time.Date(2026, time.November, 18, 18, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)
	future := now.Add(10 * time.Minute)

	tests := []struct {
		name string
		key  DigitalKey
		want bool
	}{
		{name: "free", key: DigitalKey{}, want: true},
		{name: "reservation expired", key: DigitalKey{ReservedUntil: &past}, want: true},
		{name: "reserved", key: DigitalKey{ReservedUntil: &future}, want: false},
		{name: "sold", key: DigitalKey{SoldAt: &past}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.key.IsAvailable(now); got != tt.want {
				t.Errorf("IsAvailable = %v; want %v", got, tt.want)
			}
		})
	}
}
