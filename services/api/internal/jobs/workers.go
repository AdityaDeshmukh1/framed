package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
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

type embedRequest struct {
	Text string `json:"text"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Register wires up all job handlers to the Asynq mux.
// One place, all handlers — easy to see what the worker process handles.
func (w *Workers) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(string(models.JobTypeScrapeProfile), w.handleScrapeProfile)
	mux.HandleFunc(string(models.JobTypeComputeVector), w.handleComputeVector)
	mux.HandleFunc(string(models.JobTypeEnrichFilm), w.handleEnrichFilm)
	mux.HandleFunc(string(models.JobTypeEmbedFilm), w.handleEmbedFilm)

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

	if _, err = w.pool.Exec(ctx,
		`UPDATE scrape_jobs SET status = 'scraping', films_found = $1, started_at = now() WHERE letterboxd_handle = $2`,
		len(ratings), payload.LetterboxdHandle); err != nil {
		log.Printf("[scrape] failed to update job status to scraping: %v", err)
	}

	for _, r := range ratings {
		var ratingStr string
		if r.Rating != nil {
			ratingStr = fmt.Sprintf("%.1f", *r.Rating)
		} else {
			ratingStr = "unrated"
		}
		log.Printf("[scrape] film=%s year=%d rating=%s rewatch=%v liked=%v slug=%s",
			r.Title, r.Year, ratingStr, r.Rewatch, r.Liked, r.LetterboxdSlug)

		if r.TMDBMovieID == 0 {
			log.Printf("[scrape] skipping film=%s — no TMDB ID", r.Title)
			continue
		}

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
			if _, err = w.asynqClient.Enqueue(enrichTask, asynq.TaskID("enrich:"+filmID)); err != nil && err != asynq.ErrTaskIDConflict {
				log.Printf("[scrape] failed to enqueue enrich task for film=%s err=%v", r.Title, err)
			}
		}

		// store ratings in user_ratings table
		var ratingID string
		err = w.pool.QueryRow(ctx,
			`INSERT INTO user_ratings (user_id, film_id, rating, watched_date, rewatch, liked)
VALUES ($1, $2, $3, $4, $5, $6)
     ON CONFLICT DO NOTHING
     RETURNING id`,
			payload.UserID, filmID, r.Rating, r.WatchedDate, r.Rewatch, r.Liked,
		).Scan(&ratingID)
		if err != nil && err != pgx.ErrNoRows {
			return fmt.Errorf("insert rating: %w", err)
		}
		log.Printf("[scrape] processed film=%s filmID=%s", r.Title, filmID)
	}
	log.Printf("[scrape] completed all films for user=%s", payload.UserID)

	if _, err = w.pool.Exec(ctx,
		`UPDATE scrape_jobs SET status = 'complete', films_processed = $1, completed_at = now() WHERE letterboxd_handle = $2`,
		len(ratings), payload.LetterboxdHandle); err != nil {
		log.Printf("[scrape] failed to update job status to complete: %v", err)
	}

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
		if strings.Contains(err.Error(), "not found on TMDB") {
			log.Printf("[enrich] film=%s tmdbID=%d not found on TMDB — marking as skipped", payload.FilmID, payload.TMDBMovieID)
			_, _ = w.pool.Exec(ctx,
				`UPDATE films SET embedding_computed = true WHERE id = $1`,
				payload.FilmID)
			return nil
		}
		return fmt.Errorf("enrich film %s: %w", payload.FilmID, err)
	}

	log.Printf("[enrich] film=%s keywords=%v", payload.FilmID, filmData.Keywords)

	_, err = w.pool.Exec(ctx,
		`UPDATE films 
         SET synopsis = $1, directors = $2, genres = $3, poster_path = $4, themes = $5
         WHERE id = $6`,
		filmData.Synopsis,
		filmData.Directors,
		filmData.Genres,
		filmData.PosterPath,
		filmData.Keywords,
		payload.FilmID,
	)
	if err != nil {
		return fmt.Errorf("update film %s: %w", payload.FilmID, err)
	}

	log.Printf("[enrich] completed film=%s", payload.FilmID)

	embedTask, err := NewEmbedFilmTask(payload.FilmID)
	if err != nil {
		log.Printf("[enrich] failed to create embed task for film=%s err=%v", payload.FilmID, err)
	} else {
		if _, err = w.asynqClient.Enqueue(embedTask, asynq.TaskID("embed:"+payload.FilmID)); err != nil && err != asynq.ErrTaskIDConflict {
			log.Printf("[enrich] failed to enqueue embed task for film=%s err=%v", payload.FilmID, err)
		}
	}
	return nil
}

// handleComputeVector builds a taste vector from a user's ratings.
func (w *Workers) handleComputeVector(ctx context.Context, t *asynq.Task) error {
	var payload models.ComputeVectorPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	log.Printf("[vector] computing taste vector for user=%s", payload.UserID)

	rows, err := w.pool.Query(ctx,
		`SELECT ur.rating, f.embedding::text
		 FROM user_ratings ur
		 JOIN films f ON f.id = ur.film_id
		 WHERE ur.user_id = $1 AND f.embedding IS NOT NULL`,
		payload.UserID)
	if err != nil {
		return fmt.Errorf("fetch ratings: %w", err)
	}
	defer rows.Close()

	type entry struct {
		rating    *float64
		embedding []float32
	}

	var entries []entry
	var ratingSum float64
	var ratedCount int

	for rows.Next() {
		var rating *float64
		var embStr string
		if err := rows.Scan(&rating, &embStr); err != nil {
			return fmt.Errorf("scan row: %w", err)
		}
		emb, err := parseVector(embStr)
		if err != nil {
			return fmt.Errorf("parse embedding: %w", err)
		}
		entries = append(entries, entry{rating: rating, embedding: emb})
		if rating != nil {
			ratingSum += *rating
			ratedCount++
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rows: %w", err)
	}
	if len(entries) == 0 {
		log.Printf("[vector] no embeddings found for user=%s, skipping", payload.UserID)
		return nil
	}

	var avgRating float64
	if ratedCount > 0 {
		avgRating = ratingSum / float64(ratedCount)
	}

	dim := len(entries[0].embedding)
	weights := make([]float64, len(entries))
	var absWeightSum float64
	for i, e := range entries {
		if e.rating != nil {
			weights[i] = *e.rating - avgRating
			absWeightSum += math.Abs(weights[i])
		}
	}

	result := make([]float64, dim)
	if absWeightSum == 0 {
		for _, e := range entries {
			for j, v := range e.embedding {
				result[j] += float64(v)
			}
		}
		for j := range result {
			result[j] /= float64(len(entries))
		}
	} else {
		for i, e := range entries {
			for j, v := range e.embedding {
				result[j] += weights[i] * float64(v)
			}
		}
		for j := range result {
			result[j] /= absWeightSum
		}
	}

	parts := make([]string, dim)
	for i, v := range result {
		parts[i] = fmt.Sprintf("%g", float32(v))
	}
	vecStr := "[" + strings.Join(parts, ",") + "]"

	_, err = w.pool.Exec(ctx,
		`INSERT INTO taste_vectors (user_id, vector)
		 VALUES ($1, $2::vector)
		 ON CONFLICT (user_id) DO UPDATE SET vector = EXCLUDED.vector, computed_at = now()`,
		payload.UserID, vecStr)
	if err != nil {
		return fmt.Errorf("upsert taste vector for user=%s: %w", payload.UserID, err)
	}

	log.Printf("[vector] completed user=%s", payload.UserID)
	return nil
}

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

func (w *Workers) handleEmbedFilm(ctx context.Context, t *asynq.Task) error {
	var payload models.EmbedFilmPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	log.Printf("[embed] starting embed for film=%s", payload.FilmID)

	var synopsis string
	var genres, directors, themes []string

	err := w.pool.QueryRow(ctx,
		`SELECT synopsis, genres, directors, themes FROM films 
			WHERE id = $1`,
		payload.FilmID).Scan(&synopsis, &genres, &directors, &themes)
	if err != nil {
		return fmt.Errorf("fetch film id: %w", err)
	}

	embeddingText := strings.Join(genres, " ") + ". " +
		strings.Join(directors, " ") + ". " +
		strings.Join(themes, " ") + ". " +
		synopsis

	endpoint := "http://localhost:8000/embed"

	body, err := json.Marshal(embedRequest{Text: embeddingText})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("embed service returned status %d for film=%s", resp.StatusCode, payload.FilmID)
	}

	var embedding embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedding); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	parts := make([]string, len(embedding.Embedding))
	for i, v := range embedding.Embedding {
		parts[i] = fmt.Sprintf("%g", v)
	}
	vecStr := "[" + strings.Join(parts, ",") + "]"

	_, err = w.pool.Exec(ctx,
		`UPDATE films
         SET embedding = $1::vector, embedding_computed = $2
         WHERE id = $3`,
		vecStr,
		true,
		payload.FilmID,
	)
	if err != nil {
		return fmt.Errorf("update film %s: %w", payload.FilmID, err)
	}

	log.Printf("[embed] completed film=%s", payload.FilmID)

	// Find all users who rated this film and enqueue ComputeVector for any
	// whose remaining unembedded films have hit zero.
	userRows, err := w.pool.Query(ctx,
		`SELECT DISTINCT ur.user_id FROM user_ratings ur WHERE ur.film_id = $1 AND ur.user_id IS NOT NULL`,
		payload.FilmID)
	if err != nil {
		log.Printf("[embed] failed to query users for film=%s err=%v", payload.FilmID, err)
		return nil
	}
	defer userRows.Close()

	for userRows.Next() {
		var userID string
		if err := userRows.Scan(&userID); err != nil {
			continue
		}
		var pending int
		if err := w.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM user_ratings ur
			 JOIN films f ON f.id = ur.film_id
			 WHERE ur.user_id = $1 AND f.tmdb_id != 0 AND f.embedding IS NULL`,
			userID).Scan(&pending); err != nil {
			log.Printf("[embed] failed to count pending embeddings for user=%s err=%v", userID, err)
			continue
		}
		if pending > 0 {
			continue
		}
		vectorTask, err := NewComputeVectorTask(userID)
		if err != nil {
			log.Printf("[embed] failed to create vector task for user=%s err=%v", userID, err)
			continue
		}
		if _, err = w.asynqClient.Enqueue(vectorTask, asynq.TaskID("vector:"+userID)); err != nil && err != asynq.ErrTaskIDConflict {
			log.Printf("[embed] failed to enqueue vector task for user=%s err=%v", userID, err)
		}
	}

	return nil
}
