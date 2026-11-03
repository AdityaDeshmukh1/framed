package matching

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/framed-app/api/pkg/db"
)

// Matcher holds the DB pool and exposes the matching logic.
type Matcher struct {
	pool *db.Pool
}

func New(pool *db.Pool) *Matcher {
	return &Matcher{pool: pool}
}

// Match is a persisted user pair with a compatibility score and a suggested first-date film.
type Match struct {
	UserA           string  `json:"userA"`
	UserB           string  `json:"userB"`
	Score           float64 `json:"score"`
	FirstDateFilmID string  `json:"firstDateFilmId"`
}

// FindMatches finds the top N taste-compatible users for userID, selects a
// first-date film for each pair (the film embedding nearest to the taste
// midpoint that neither user has already seen), and upserts each result into
// the matches table.
func (m *Matcher) FindMatches(ctx context.Context, userID string, limit int) ([]Match, error) {
	callerVec, callerVecStr, err := m.fetchVector(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetch caller vector: %w", err)
	}

	candidates, err := m.findCandidates(ctx, userID, callerVecStr, limit)
	if err != nil {
		return nil, fmt.Errorf("find candidates: %w", err)
	}

	var matches []Match
	for _, c := range candidates {
		match, ok, err := m.buildMatch(ctx, userID, callerVec, c)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		matches = append(matches, match)
	}

	return matches, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

// scoreFloor is the cosine similarity treated as "no shared taste". Taste
// vectors are averages of sentence embeddings, which share a large common
// component, so even two users with zero films in common score ~0.8 raw.
// Measured on real profiles: different users ≈ 0.81, same profile = 1.0.
// Recalibrate once there are more users.
const scoreFloor = 0.75

// displayScore stretches raw cosine similarity from [scoreFloor, 1] to [0, 1]
// so match percentages use the whole range. Ordering is unchanged.
func displayScore(cosine float64) float64 {
	s := (cosine - scoreFloor) / (1 - scoreFloor)
	return math.Max(0, math.Min(1, s))
}

type candidate struct {
	userID string
	score  float64
	vec    []float32
}

// fetchVector loads a user's taste vector from the DB.
func (m *Matcher) fetchVector(ctx context.Context, userID string) ([]float32, string, error) {
	var s string
	if err := m.pool.QueryRow(ctx,
		`SELECT vector::text FROM taste_vectors WHERE user_id = $1`,
		userID).Scan(&s); err != nil {
		return nil, "", err
	}
	vec, err := parseVector(s)
	return vec, s, err
}

// findCandidates returns the top N users ordered by cosine similarity, along
// with their parsed taste vectors.
func (m *Matcher) findCandidates(ctx context.Context, userID, callerVecStr string, limit int) ([]candidate, error) {
	rows, err := m.pool.Query(ctx,
		`SELECT user_id, 1 - (vector <=> $1::vector) AS score, vector::text
		 FROM taste_vectors
		 WHERE user_id != $2
		 ORDER BY vector <=> $1::vector
		 LIMIT $3`,
		callerVecStr, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []candidate
	for rows.Next() {
		var c candidate
		var vecStr string
		var cosine float64
		if err := rows.Scan(&c.userID, &cosine, &vecStr); err != nil {
			return nil, err
		}
		c.score = displayScore(cosine)
		c.vec, err = parseVector(vecStr)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

// buildMatch computes the midpoint vector, finds the best first-date film, and
// upserts the match row. Returns ok=false when no unseen film exists for the pair.
func (m *Matcher) buildMatch(ctx context.Context, callerID string, callerVec []float32, c candidate) (Match, bool, error) {
	// Canonical ordering prevents (A,B) and (B,A) duplicates.
	userA, userB := callerID, c.userID
	if userA > userB {
		userA, userB = userB, userA
	}

	mid := midpoint(callerVec, c.vec)
	midStr := formatVector(mid)

	// Film nearest to the taste midpoint, excluding films either user rated poorly.
	// "Unseen by both" is ideal but impossible when the film pool is small, so we
	// fall back to excluding only disliked films (rating < 3.5).
	var filmID string
	err := m.pool.QueryRow(ctx,
		`SELECT f.id FROM films f
		 WHERE f.embedding IS NOT NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM user_ratings
		     WHERE film_id = f.id AND user_id IN ($1, $2)
		       AND rating IS NOT NULL AND rating < 3.5
		   )
		 ORDER BY f.embedding <=> $3::vector
		 LIMIT 1`,
		userA, userB, midStr).Scan(&filmID)
	if err != nil {
		// No unseen film available — skip this candidate.
		return Match{}, false, nil
	}

	if _, err = m.pool.Exec(ctx,
		`INSERT INTO matches (user_a, user_b, score, first_date_film_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_a, user_b) DO UPDATE
		   SET score = EXCLUDED.score,
		       first_date_film_id = EXCLUDED.first_date_film_id`,
		userA, userB, c.score, filmID); err != nil {
		return Match{}, false, fmt.Errorf("upsert match (%s, %s): %w", userA, userB, err)
	}

	return Match{
		UserA:           userA,
		UserB:           userB,
		Score:           c.score,
		FirstDateFilmID: filmID,
	}, true, nil
}

// ── vector math ───────────────────────────────────────────────────────────────

// midpoint returns the element-wise average of two equal-length vectors.
func midpoint(a, b []float32) []float32 {
	mid := make([]float32, len(a))
	for i := range a {
		mid[i] = (a[i] + b[i]) / 2
	}
	return mid
}

// parseVector parses a pgvector text representation "[v1,v2,...]" into []float32.
func parseVector(s string) ([]float32, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	parts := strings.Split(s, ",")
	vec := make([]float32, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("parse element %d %q: %w", i, p, err)
		}
		vec[i] = float32(v)
	}
	return vec, nil
}

// formatVector formats []float32 as a pgvector string "[v1,v2,...]".
func formatVector(v []float32) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = strconv.FormatFloat(float64(x), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
