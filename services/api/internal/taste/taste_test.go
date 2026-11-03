package taste

import (
	"math"
	"testing"
)

func ptr(f float64) *float64 { return &f }

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func TestPositiveWeightsStayPositive(t *testing.T) {
	cases := []struct {
		name     string
		e        Entry
		baseline float64
		want     float64
	}{
		{"unrated", Entry{}, 3.5, 1.0},
		{"above baseline", Entry{Rating: ptr(5)}, 3.5, 2.5},
		{"far below floors at 0.1", Entry{Rating: ptr(0.5)}, 4.0, 0.1},
		{"liked adds one", Entry{Rating: ptr(3.5), Liked: true}, 3.5, 2.0},
	}
	for _, c := range cases {
		if got := Weight(c.e, c.baseline, false); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSignedWeightsGoNegative(t *testing.T) {
	if got := Weight(Entry{Rating: ptr(1)}, 3.5, true); got >= 0 {
		t.Errorf("disliked film weight = %v, want < 0", got)
	}
	if got := Weight(Entry{}, 3.5, true); got != 0.3 {
		t.Errorf("unrated film weight = %v, want 0.3", got)
	}
}

func TestVectorIsUnitLength(t *testing.T) {
	v := Vector([]Entry{
		{Rating: ptr(5), Embedding: []float32{3, 0, 0}},
		{Rating: ptr(1), Embedding: []float32{0, 4, 0}},
		{Embedding: []float32{0, 0, 2}},
	}, V1)
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	if math.Abs(math.Sqrt(s)-1) > 1e-6 {
		t.Fatalf("length = %v, want 1", math.Sqrt(s))
	}
}

// Two users with the same films and similar ratings must stay close; the old
// mean-centred v0 weights made them near-orthogonal.
func TestSimilarUsersScoreHigh(t *testing.T) {
	films := [][]float32{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}}
	alice := []Entry{{Rating: ptr(5), Embedding: films[0]}, {Rating: ptr(4.5), Embedding: films[1]}, {Rating: ptr(2), Embedding: films[2]}}
	bob := []Entry{{Rating: ptr(4.5), Embedding: films[0]}, {Rating: ptr(5), Embedding: films[1]}, {Rating: ptr(1.5), Embedding: films[2]}}
	carol := []Entry{{Rating: ptr(5), Embedding: films[3]}, {Rating: ptr(2), Embedding: films[2]}}

	ab := cosine(Vector(alice, V1), Vector(bob, V1))
	ac := cosine(Vector(alice, V1), Vector(carol, V1))
	if ab < 0.9 {
		t.Errorf("alice~bob = %.3f, want >= 0.9", ab)
	}
	if ac >= ab {
		t.Errorf("alice~carol = %.3f should be lower than alice~bob = %.3f", ac, ab)
	}
}

// Loving a film the crowd loves should count for less than loving one the
// crowd is lukewarm on.
func TestCrowdBaselineRewardsDivisiveLoves(t *testing.T) {
	consensus := Entry{Rating: ptr(4.5), CrowdMean: ptr(4.4)}
	divisive := Entry{Rating: ptr(4.5), CrowdMean: ptr(3.0)}
	b := baselines([]Entry{consensus, divisive}, Crowd)
	if Weight(divisive, b[1], false) <= Weight(consensus, b[0], false) {
		t.Errorf("divisive love should outweigh consensus love: %v vs %v", Weight(divisive, b[1], false), Weight(consensus, b[0], false))
	}
}

func TestCenterRemovesSharedComponent(t *testing.T) {
	action := []float32{11, 10, 10}
	romance := []float32{10, 11, 10}
	center := []float32{10.5, 10.5, 10}
	u1 := []Entry{{Rating: ptr(5), Embedding: action}}
	u2 := []Entry{{Rating: ptr(5), Embedding: romance}}
	if raw := cosine(Vector(u1, V1), Vector(u2, V1)); raw < 0.99 {
		t.Fatalf("precondition: raw similarity %.3f should be ~1", raw)
	}
	opts := Options{Center: center}
	if c := cosine(Vector(u1, opts), Vector(u2, opts)); c > 0 {
		t.Errorf("centred similarity = %.3f, want <= 0", c)
	}
}

func TestEmpty(t *testing.T) {
	if v := Vector(nil, V1); v != nil {
		t.Errorf("got %v, want nil", v)
	}
}

func TestSignedFallsBackWhenAllRatingsEqual(t *testing.T) {
	center := []float32{0.5, 0.5, 0}
	// Not symmetric about the centre (that would cancel under any weights).
	entries := []Entry{{Rating: ptr(3), Embedding: []float32{1, 0, 0}}, {Rating: ptr(3), Embedding: []float32{0, 0, 1}}}
	if v := Vector(entries, Production(center)); v == nil {
		t.Fatal("expected a fallback vector, got nil")
	}
}

func TestProductionSeparatesOppositeTastes(t *testing.T) {
	center := []float32{0.5, 0.5, 0}
	action, romance := []float32{1, 0, 0}, []float32{0, 1, 0}
	a := []Entry{{Rating: ptr(5), Embedding: action}, {Rating: ptr(1), Embedding: romance}}
	b := []Entry{{Rating: ptr(1), Embedding: action}, {Rating: ptr(5), Embedding: romance}}
	if c := cosine(Vector(a, Production(center)), Vector(b, Production(center))); c > -0.9 {
		t.Errorf("opposite tastes cosine = %.3f, want ≈ -1", c)
	}
}
