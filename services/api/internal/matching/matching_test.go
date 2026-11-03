package matching

import "testing"

func TestDisplayScore(t *testing.T) {
	cases := []struct{ cosine, want float64 }{
		{scoreCeiling, 1.0},
		{1.0, 1.0}, // above ceiling clamps to 1
		{scoreFloor, 0.0},
		{-0.9, 0.0}, // opposite tastes clamp to 0
		{(scoreFloor + scoreCeiling) / 2, 0.5},
	}
	for _, c := range cases {
		if got := displayScore(c.cosine); got < c.want-1e-9 || got > c.want+1e-9 {
			t.Errorf("displayScore(%v) = %v, want %v", c.cosine, got, c.want)
		}
	}
}
