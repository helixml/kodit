package persistence

import "testing"

func TestProbeCount(t *testing.T) {
	tests := []struct {
		name string
		rows int64
		want int
	}{
		{"zero rows", 0, 10},
		{"one row", 1, 10},
		{"ten rows", 10, 10},
		{"100 rows (lists=10)", 100, 10},
		{"1000 rows (lists=100)", 1000, 10},
		{"10000 rows (lists=1000)", 10000, 31},
		{"100000 rows (lists=10000)", 100000, 100},
		{"1000000 rows (lists=100000)", 1000000, 316},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := probeCount(tt.rows)
			if got != tt.want {
				t.Errorf("probeCount(%d) = %d, want %d", tt.rows, got, tt.want)
			}
		})
	}
}
