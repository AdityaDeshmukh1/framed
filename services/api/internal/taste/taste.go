// Package taste turns a user's rated films into a taste vector: a weighted
// combination of the films' embeddings, normalised to unit length. Matching
// compares taste vectors by cosine similarity.
//
// The weighting is the whole game, and is chosen by measurement
// (go run ./cmd/eval), not taste. Options select between the variants.
package taste

import "math"

// Entry is one film in a user's history.
type Entry struct {
	Rating    *float64 // 0.5–5, nil when the user logged it without rating
	Liked     bool
	Embedding []float32
	// CrowdMean is the film's average rating across everyone (film_stats),
	// nil when unknown.
	CrowdMean *float64
}

// Baseline is what a rating is compared against.
type Baseline int

const (
	// UserMean: how much more you liked it than your average film (v1).
	UserMean Baseline = iota
	// Crowd: how much more you liked it than people usually do, after
	// allowing for how generous you are overall. Loving a film everyone
	// loves says little; loving a divisive one says a lot.
	Crowd
)

// Options selects the weighting.
type Options struct {
	Baseline Baseline
	// Center, when set, is subtracted from every embedding first. It should
	// be a stable reference such as the catalogue's popularity-weighted mean
	// embedding, NOT the mean of a handful of users (that is degenerate).
	Center []float32
	// Signed lets disliked films push the vector away (negative weight).
	// Only meaningful with Center; without it, opposite weights cancel.
	Signed bool
}

// V1 is the original weighting: positive, relative to the user's mean.
var V1 = Options{Baseline: UserMean}

// Production is the weighting the pipeline uses: subtract the catalogue mean
// embedding, then weight each film by how far your rating is from your own
// average, so disliked films push the vector away.
//
// Measured with cmd/eval on 3,461 MovieLens users / 40k user pairs, against
// V1: match fidelity (Spearman with real rating agreement) 0.199 → 0.467,
// held-out taste capture AUC 0.566 → 0.661, quiz-sized (10 film) profiles
// 0.075 → 0.176, and pairwise cosine spread std 0.033 → 0.182 (users no
// longer all look ~94% alike). A crowd baseline scored worse (0.321).
func Production(catalogueMean []float32) Options {
	return Options{Baseline: UserMean, Center: catalogueMean, Signed: true}
}

// Vector returns the unit-length taste vector, or nil if there is nothing to
// build one from.
func Vector(entries []Entry, opts Options) []float32 {
	if len(entries) == 0 {
		return nil
	}
	base := baselines(entries, opts.Baseline)
	dim := len(entries[0].Embedding)
	acc := make([]float64, dim)
	for i, e := range entries {
		w := weight(e, base[i], opts.Signed)
		for d, x := range e.Embedding {
			if opts.Center != nil {
				x -= opts.Center[d]
			}
			acc[d] += w * float64(x)
		}
	}
	v := normalize(acc)
	if v == nil && opts.Signed {
		// Every rating equals the user's mean (e.g. all "it's fine"), so all
		// signed weights are 0. Fall back to positive weights in the same
		// centred space so the vector stays comparable with everyone else's.
		opts.Signed = false
		return Vector(entries, opts)
	}
	return v
}

// Weight is exported for tests and the genre/director summaries.
func Weight(e Entry, baseline float64, signed bool) float64 { return weight(e, baseline, signed) }

func weight(e Entry, baseline float64, signed bool) float64 {
	residual := 0.0
	if e.Rating != nil {
		residual = *e.Rating - baseline
	}
	like := 0.0
	if e.Liked {
		like = 1
	}
	if signed {
		// A logged-but-unrated film is mild evidence you chose to watch it.
		if e.Rating == nil && !e.Liked {
			return 0.3
		}
		return residual + 0.5*like
	}
	return math.Max(1+residual+like, 0.1)
}

// baselines returns, per entry, the rating we'd expect from this user.
func baselines(entries []Entry, b Baseline) []float64 {
	var sum, n, crowdSum, crowdN float64
	for _, e := range entries {
		if e.Rating == nil {
			continue
		}
		sum += *e.Rating
		n++
		if e.CrowdMean != nil {
			crowdSum += *e.Rating - *e.CrowdMean
			crowdN++
		}
	}
	userMean := 3.0
	if n > 0 {
		userMean = sum / n
	}
	// How much more generous than the crowd this user is, shrunk towards 0
	// when we have few films to judge by.
	userBias := 0.0
	if crowdN > 0 {
		userBias = crowdSum / (crowdN + 5)
	}
	out := make([]float64, len(entries))
	for i, e := range entries {
		if b == Crowd && e.CrowdMean != nil {
			out[i] = *e.CrowdMean + userBias
		} else {
			out[i] = userMean
		}
	}
	return out
}

// Confidence is a 0–1 measure of how settled a profile built from n rated
// films is. Measured: match scores move ±29 points at 10 films, ±7 at 30.
func Confidence(rated int) float64 {
	return math.Min(1, float64(rated)/30)
}

func normalize(v []float64) []float32 {
	var s float64
	for _, x := range v {
		s += x * x
	}
	if s == 0 {
		return nil
	}
	n := math.Sqrt(s)
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(x / n)
	}
	return out
}
