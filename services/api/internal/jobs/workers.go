package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/framed-app/api/internal/enrichment"
	"github.com/framed-app/api/internal/scraper"
	"github.com/framed-app/api/pkg/models"
	"github.com/hibiken/asynq"
)

// Workers holds all dependencies the job handlers need.
// This is dependency injection — handlers get what they need
// through this struct, not through global variables.
type Workers struct {
	scraper  scraper.ProfileImporter
	enricher *enrichment.TMDBClient
}

func NewWorkers(scraper scraper.ProfileImporter, enricher *enrichment.TMDBClient) *Workers {
	return &Workers{
		scraper:  scraper,
		enricher: enricher,
	}
}

// Register wires up all job handlers to the Asynq mux.
// One place, all handlers — easy to see what the worker process handles.
func (w *Workers) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(string(models.JobTypeScrapeProfile), w.handleScrapeProfile)
	mux.HandleFunc(string(models.JobTypeComputeVector), w.handleComputeVector)
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

	// TODO: store ratings in user_ratings table
	// TODO: enqueue EnrichFilm job for each unseen film
	// TODO: enqueue ComputeVector job when enrichment completes

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
