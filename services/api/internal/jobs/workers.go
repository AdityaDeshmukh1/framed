package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/framed-app/api/internal/enrichment"
	"github.com/framed-app/api/internal/scraper"
	"github.com/framed-app/api/pkg/db"
	"github.com/framed-app/api/pkg/models"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
)

// Workers holds all dependencies the job handlers need.
// This is dependency injection — handlers get what they need
// through this struct, not through global variables.
type Workers struct {
	scraper     scraper.ProfileImporter
	asynqClient *asynq.Client
	enricher    *enrichment.TMDBClient
	pool        *db.Pool
}

func NewWorkers(scraper scraper.ProfileImporter, enricher *enrichment.TMDBClient, pool *db.Pool, asynqClient *asynq.Client) *Workers {
	return &Workers{
		scraper:     scraper,
		enricher:    enricher,
		pool:        pool,
		asynqClient: asynqClient,
	}
}

// Register wires up all job handlers to the Asynq mux.
// One place, all handlers — easy to see what the worker process handles.
func (w *Workers) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(string(models.JobTypeScrapeProfile), w.handleScrapeProfile)
	mux.HandleFunc(string(models.JobTypeComputeVector), w.handleComputeVector)
	mux.HandleFunc(string(models.JobTypeEnrichFilm), w.handleEnrichFilm)
}

// handleScrapeProfile scrapes a user's Letterboxd profile and
// enqueues an EnrichFilm Simjob for each film found.
func (w *Workers) handleScrapeProfile(ctx context.Context, t *asynq.Task) error {
	// deserialise the payload
	var payload models.ScrapeProfilePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	log.Printf("[scrape] starting profile scrape for user=%s handle=%s",
		payload.UserID, payload.LetterboxdHandle)

	// scrape letterboxd
	ratings, err := w.scraper.ImportProfile(ctx, payload.LetterboxdHandle)
	if err != nil {
		return fmt.Errorf("scrape profile: %w", err)
	}

	log.Printf("[scrape] found %d films for user=%s", len(ratings), payload.UserID)

	for _, r := range ratings {
		var ratingStr string
		if r.Rating != nil {
			ratingStr = fmt.Sprintf("%.1f", *r.Rating)
		} else {
			ratingStr = "unrated"
		}
		log.Printf("[scrape] film=%s year=%d rating=%s rewatch=%v liked=%v slug=%s",
			r.Title, r.Year, ratingStr, r.Rewatch, r.Liked, r.LetterboxdSlug)

		// extract slug from URL: https://letterboxd.com/user/film/slug/ -> slug
		parts := strings.Split(strings.TrimSuffix(r.LetterboxdSlug, "/"), "/")
		slug := parts[len(parts)-1]

		// Write or Fetch film from the films table
		var filmID string
		err = w.pool.QueryRow(ctx,
			`INSERT INTO films (tmdb_id, letterboxd_slug, title, year)
VALUES ($1, $2, $3, $4)
ON CONFLICT (tmdb_id) DO NOTHING
RETURNING id`,
			r.TMDBMovieID, slug, r.Title, r.Year).Scan(&filmID)

		if err == pgx.ErrNoRows {
			err = w.pool.QueryRow(ctx,
				`SELECT id FROM films 
			WHERE tmdb_id = $1`,
				r.TMDBMovieID).Scan(&filmID)
			if err != nil {
				return fmt.Errorf("fetch film id: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("upsert film: %w", err)
		}

		enrichTask, err := NewEnrichFilmTask(filmID, r.TMDBMovieID, slug)
		if err != nil {
			log.Printf("[scrape] failed to create enrich task for film=%s err=%v", r.Title, err)
		} else {
			if _, err = w.asynqClient.Enqueue(enrichTask); err != nil {
				log.Printf("[scrape] failed to enqueue enrich task for film=%s err=%v", r.Title, err)
			}
		}

		// store ratings in user_ratings table
		var ratingID string
		err = w.pool.QueryRow(ctx,
			`INSERT INTO user_ratings (film_id, rating, watched_date, rewatch, liked)
VALUES ($1, $2, $3, $4, $5)
     ON CONFLICT DO NOTHING
     RETURNING id`,
			filmID, r.Rating, r.WatchedDate, r.Rewatch, r.Liked,
		).Scan(&ratingID)
		if err != nil && err != pgx.ErrNoRows {
			return fmt.Errorf("insert rating: %w", err)
		}
		log.Printf("[scrape] processed film=%s filmID=%s", r.Title, filmID)
	}
	log.Printf("[scrape] completed all films for user=%s", payload.UserID)

	// TODO: enqueue EnrichFilm job for each unseen film
	// TODO: enqueue ComputeVector job when enrichment completes

	return nil
}

func (w *Workers) handleEnrichFilm(ctx context.Context, t *asynq.Task) error {
	var payload models.EnrichFilmPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	log.Printf("[enrich] enriching film=%s tmdbID=%d", payload.FilmID, payload.TMDBMovieID)

	// retry up to 3 times — ISP resets are intermittent
	var filmData *enrichment.FilmData
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		filmData, err = w.enricher.GetFilmDetails(ctx, payload.TMDBMovieID)
		if err == nil {
			break
		}
		log.Printf("[enrich] attempt %d failed for tmdbID=%d err=%v", attempt+1, payload.TMDBMovieID, err)
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("enrich film %s: %w", payload.FilmID, err)
	}

	_, err = w.pool.Exec(ctx,
		`UPDATE films 
         SET synopsis = $1, directors = $2, genres = $3, poster_path = $4
         WHERE id = $5`,
		filmData.Synopsis,
		filmData.Directors,
		filmData.Genres,
		filmData.PosterPath,
		payload.FilmID,
	)
	if err != nil {
		return fmt.Errorf("update film %s: %w", payload.FilmID, err)
	}

	log.Printf("[enrich] completed film=%s", payload.FilmID)
	return nil
}

// handleComputeVector builds a taste vector from a user's ratings.
func (w *Workers) handleComputeVector(ctx context.Context, t *asynq.Task) error {
	var payload models.ComputeVectorPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	log.Printf("[vector] computing taste vector for user=%s", payload.UserID)

	// TODO: fetch user ratings from DB
	// TODO: compute weighted taste vector
	// TODO: store in taste_vectors table
	// TODO: enqueue GenerateSoul job

	return nil
}
