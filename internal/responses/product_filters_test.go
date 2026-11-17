package responses

import "testing"

func TestRoundUpToWholeEuro(t *testing.T) {
	tests := []struct {
		cents int
		want  int
	}{
		{0, 0},
		{1, 100},
		{99, 100},
		{100, 100},
		{101, 200},
		{8999, 9000},
		{9000, 9000},
	}

	for _, tt := range tests {
		if got := roundUpToWholeEuro(tt.cents); got != tt.want {
			t.Errorf("roundUpToWholeEuro(%d) = %d; want %d", tt.cents, got, tt.want)
		}
	}
}
