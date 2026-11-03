package jobs

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

func length(v []float32) float64 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	return math.Sqrt(s)
}

func TestFilmWeightStaysPositive(t *testing.T) {
	cases := []struct {
		name string
		e    tasteEntry
		avg  float64
		want float64
	}{
		{"unrated", tasteEntry{}, 3.5, 1.0},
		{"above average", tasteEntry{rating: ptr(5)}, 3.5, 2.5},
		{"far below average floors at 0.1", tasteEntry{rating: ptr(0.5)}, 4.0, 0.1},
		{"liked adds one", tasteEntry{rating: ptr(3.5), liked: true}, 3.5, 2.0},
	}
	for _, c := range cases {
		if got := filmWeight(c.e, c.avg); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestComputeTasteVectorIsUnitLength(t *testing.T) {
	entries := []tasteEntry{
		{rating: ptr(5), embedding: []float32{3, 0, 0}},
		{rating: ptr(1), embedding: []float32{0, 4, 0}},
		{embedding: []float32{0, 0, 2}},
	}
	v := computeTasteVector(entries, nil)
	if got := length(v); math.Abs(got-1) > 1e-6 {
		t.Fatalf("length = %v, want 1", got)
	}
}

// The old mean-centred weights made opposite ratings cancel and produced a
// near-zero, near-orthogonal vector. Positive weights must keep two users with
// the same films and similar ratings close together.
func TestSimilarUsersScoreHigh(t *testing.T) {
	films := [][]float32{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}}
	alice := []tasteEntry{
		{rating: ptr(5), embedding: films[0]},
		{rating: ptr(4.5), embedding: films[1]},
		{rating: ptr(2), embedding: films[2]},
	}
	bob := []tasteEntry{
		{rating: ptr(4.5), embedding: films[0]},
		{rating: ptr(5), embedding: films[1]},
		{rating: ptr(1.5), embedding: films[2]},
	}
	carol := []tasteEntry{
		{rating: ptr(5), embedding: films[3]},
		{rating: ptr(2), embedding: films[2]},
	}

	ab := cosine(computeTasteVector(alice, nil), computeTasteVector(bob, nil))
	ac := cosine(computeTasteVector(alice, nil), computeTasteVector(carol, nil))
	if ab < 0.9 {
		t.Errorf("alice~bob = %.3f, want >= 0.9", ab)
	}
	if ac >= ab {
		t.Errorf("alice~carol = %.3f should be lower than alice~bob = %.3f", ac, ab)
	}
}

// Every embedding sharing a big common component makes all users look alike.
// Subtracting the global mean must remove it and separate different tastes.
func TestCenteringRemovesSharedComponent(t *testing.T) {
	shared := []float32{10, 10, 10}
	add := func(a, b []float32) []float32 {
		out := make([]float32, len(a))
		for i := range a {
			out[i] = a[i] + b[i]
		}
		return out
	}
	action := add(shared, []float32{1, 0, 0})
	romance := add(shared, []float32{0, 1, 0})
	mean := []float32{10.5, 10.5, 10}

	u1 := []tasteEntry{{rating: ptr(5), embedding: action}}
	u2 := []tasteEntry{{rating: ptr(5), embedding: romance}}

	raw := cosine(computeTasteVector(u1, nil), computeTasteVector(u2, nil))
	centred := cosine(computeTasteVector(u1, mean), computeTasteVector(u2, mean))
	if raw < 0.99 {
		t.Fatalf("precondition: raw similarity %.3f should be ~1", raw)
	}
	if centred > 0 {
		t.Errorf("centred similarity = %.3f, want <= 0 for opposite tastes", centred)
	}
}

func TestComputeTasteVectorEmpty(t *testing.T) {
	if v := computeTasteVector(nil, nil); v != nil {
		t.Errorf("got %v, want nil", v)
	}
}
