package jobs

import "math"

// tasteEntry is one logged film as seen by the taste-vector computation.
type tasteEntry struct {
	rating    *float64 // nil when the film was logged without a rating
	liked     bool
	embedding []float32
}

// filmWeight scores how strongly a film should pull the taste vector.
//
// Every logged film counts (watching it is signal), above-average ratings and
// likes count more, and below-average ratings shrink towards — but never past —
// a small positive floor. Weights must stay positive: negative weights turn the
// taste vector into a "liked minus disliked" difference vector, and two such
// vectors are close to orthogonal for almost any pair of users, which is what
// produced ~1% match scores before.
func filmWeight(e tasteEntry, avgRating float64) float64 {
	w := 1.0
	if e.rating != nil {
		w += *e.rating - avgRating
	}
	if e.liked {
		w += 1.0
	}
	return math.Max(w, 0.1)
}

// computeTasteVector returns the unit-length, weighted average of the user's
// film embeddings after subtracting globalMean.
//
// Centering matters as much as the weights: sentence embeddings of film
// descriptions share a large common component, so raw averages of any two
// users' films have cosine similarity ~0.9 regardless of taste. Subtracting
// the mean embedding of all films leaves only what is distinctive about this
// user's films. globalMean may be nil to skip centering.
//
// Returns nil when there are no entries or the result has zero length.
func computeTasteVector(entries []tasteEntry, globalMean []float32) []float32 {
	if len(entries) == 0 {
		return nil
	}

	var ratingSum float64
	var ratedCount int
	for _, e := range entries {
		if e.rating != nil {
			ratingSum += *e.rating
			ratedCount++
		}
	}
	var avgRating float64
	if ratedCount > 0 {
		avgRating = ratingSum / float64(ratedCount)
	}

	dim := len(entries[0].embedding)
	sum := make([]float64, dim)
	for _, e := range entries {
		w := filmWeight(e, avgRating)
		for j, v := range e.embedding {
			c := float64(v)
			if globalMean != nil {
				c -= float64(globalMean[j])
			}
			sum[j] += w * c
		}
	}

	// Cosine similarity ignores magnitude, so dividing by the weight sum is
	// unnecessary — normalising to unit length is enough and keeps stored
	// vectors comparable with plain dot products too.
	var norm float64
	for _, v := range sum {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return nil
	}

	out := make([]float32, dim)
	for j, v := range sum {
		out[j] = float32(v / norm)
	}
	return out
}
