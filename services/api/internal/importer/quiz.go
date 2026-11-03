package importer

import (
	"context"
	"errors"
	"fmt"
	"math"
)

// SourceQuiz is the onboarding rating quiz: the client submits the films the
// user rated, and this importer validates them against the catalogue.
const SourceQuiz Source = "quiz"

// MaxQuizRatings caps one submission; the quiz itself stops well before.
const MaxQuizRatings = 120

// Submitted is one quiz answer.
type Submitted struct {
	TMDBMovieID int      `json:"tmdb_id"`
	Rating      *float32 `json:"rating,omitempty"` // 0.5–5 in half stars
	Liked       bool     `json:"liked,omitempty"`
}

// CatalogueFilm is what the quiz importer needs to know about a film.
type CatalogueFilm struct {
	Title string
	Year  int
}

// CatalogueLookup returns the catalogue entries for the given TMDB ids;
// ids not in the catalogue are simply absent from the result.
type CatalogueLookup func(ctx context.Context, tmdbIDs []int) (map[int]CatalogueFilm, error)

// Quiz imports quiz answers. Quiz cards only ever show catalogue films, so
// anything else is rejected rather than enriched: it keeps onboarding instant
// and stops a client from making us fetch arbitrary TMDB ids.
type Quiz struct {
	lookup CatalogueLookup
}

func NewQuiz(lookup CatalogueLookup) *Quiz { return &Quiz{lookup: lookup} }

func (q *Quiz) Source() Source { return SourceQuiz }

// ErrInvalidSubmission marks a client error in quiz answers.
var ErrInvalidSubmission = errors.New("invalid quiz submission")

// Validate checks answers without touching the catalogue; the HTTP handler
// calls it so bad input is a 400, not a failed job.
func Validate(answers []Submitted) error {
	if len(answers) == 0 {
		return fmt.Errorf("%w: no ratings", ErrInvalidSubmission)
	}
	if len(answers) > MaxQuizRatings {
		return fmt.Errorf("%w: at most %d ratings", ErrInvalidSubmission, MaxQuizRatings)
	}
	for _, a := range answers {
		if a.TMDBMovieID <= 0 {
			return fmt.Errorf("%w: bad tmdb_id %d", ErrInvalidSubmission, a.TMDBMovieID)
		}
		if a.Rating != nil {
			r := float64(*a.Rating)
			if r < 0.5 || r > 5 || math.Mod(r*2, 1) != 0 {
				return fmt.Errorf("%w: rating %v for tmdb_id %d must be 0.5–5 in half stars", ErrInvalidSubmission, r, a.TMDBMovieID)
			}
		}
	}
	return nil
}

func (q *Quiz) Import(ctx context.Context, req Request) ([]Film, error) {
	if err := Validate(req.Submitted); err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(req.Submitted))
	for _, a := range req.Submitted {
		ids = append(ids, a.TMDBMovieID)
	}
	known, err := q.lookup(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("catalogue lookup: %w", err)
	}

	seen := map[int]bool{}
	var films []Film
	for _, a := range req.Submitted {
		cf, ok := known[a.TMDBMovieID]
		if !ok || seen[a.TMDBMovieID] {
			continue // not a quiz film, or answered twice (keep the first)
		}
		seen[a.TMDBMovieID] = true
		films = append(films, Film{
			TMDBMovieID: a.TMDBMovieID,
			Title:       cf.Title,
			Year:        cf.Year,
			Rating:      a.Rating,
			Liked:       a.Liked,
		})
	}
	if len(films) == 0 {
		return nil, fmt.Errorf("%w: none of the films are in the catalogue", ErrInvalidSubmission)
	}
	return films, nil
}
