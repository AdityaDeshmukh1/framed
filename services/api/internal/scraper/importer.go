package scraper

import (
	"context"
	"time"
)

// RawRating is what the scraper extracts from Letterboxd before TMDB enrichment.
// TMDBMovieID is resolved by workers.go via TMDB title+year search after scraping.
type RawRating struct {
	TMDBMovieID    int
	LetterboxdSlug string
	Title          string
	Year           int
	Rating         *float32
	Liked          bool
	WatchedDate    *time.Time
	Rewatch        bool
}

type ProfileImporter interface {
	ImportProfile(ctx context.Context, handle string) ([]RawRating, error)
}
