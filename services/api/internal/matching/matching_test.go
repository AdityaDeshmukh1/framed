package matching

import "testing"

func TestDisplayScore(t *testing.T) {
	cases := []struct{ cosine, want float64 }{
		{1.0, 1.0},
		{scoreFloor, 0.0},
		{0.5, 0.0},  // below floor clamps to 0
		{-0.3, 0.0}, // negative cosine clamps to 0
		{0.875, 0.5},
	}
	for _, c := range cases {
		if got := displayScore(c.cosine); got < c.want-1e-9 || got > c.want+1e-9 {
			t.Errorf("displayScore(%v) = %v, want %v", c.cosine, got, c.want)
		}
	}
}
