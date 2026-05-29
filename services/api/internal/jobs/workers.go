package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/framed-app/api/internal/enrichment"
	"github.com/framed-app/api/internal/llm"
	"github.com/framed-app/api/internal/scraper"
	"github.com/framed-app/api/pkg/db"
	"github.com/framed-app/api/pkg/models"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
)

// Workers holds all dependencies the job handlers need.
type Workers struct {
	scraper      scraper.ProfileImporter
	asynqClient  *asynq.Client
	enricher     *enrichment.TMDBClient
	pool         *db.Pool
	anthropicKey string
	mlServiceURL string
	// tmdbSem caps concurrent TMDB requests — CloudFront resets connections
	// when too many hit simultaneously from the same IP.
	tmdbSem chan struct{}
}

func NewWorkers(scraper scraper.ProfileImporter, enricher *enrichment.TMDBClient, pool *db.Pool, asynqClient *asynq.Client, anthropicKey, mlServiceURL string) *Workers {
	return &Workers{
		scraper:      scraper,
		enricher:     enricher,
		pool:         pool,
		asynqClient:  asynqClient,
		anthropicKey: anthropicKey,
		mlServiceURL: mlServiceURL,
		tmdbSem:      make(chan struct{}, 5),
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
	mux.HandleFunc(string(models.JobTypeGenerateBlurbs), w.handleGenerateBlurbs)
}

// handleScrapeProfile scrapes a user's Letterboxd profile and
// enqueues an EnrichFilm Simjob for each film found.
func (w *Workers) handleScrapeProfile(ctx context.Context, t *asynq.Task) error {
	// deserialise the payload
	var payload models.ScrapeProfilePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	scrapeStart := time.Now()
	log.Printf("[scrape] job=%s user=%s handle=%s starting", payload.JobID, payload.UserID, payload.LetterboxdHandle)

	// Mark as scraping immediately so the UI transitions out of 'queued'
	if _, err := w.pool.Exec(ctx,
		`UPDATE scrape_jobs SET status = 'scraping', started_at = now() WHERE id = $1`,
		payload.JobID); err != nil {
		log.Printf("[scrape] job=%s failed to update status to scraping: %v", payload.JobID, err)
	}

	// scrape letterboxd
	ratings, err := w.scraper.ImportProfile(ctx, payload.LetterboxdHandle)
	if err != nil {
		return fmt.Errorf("scrape profile: %w", err)
	}

	log.Printf("[scrape] job=%s scraped %d films in %s", payload.JobID, len(ratings), time.Since(scrapeStart).Round(time.Millisecond))

	// Update films_found count now that we know the total
	if _, err := w.pool.Exec(ctx,
		`UPDATE scrape_jobs SET films_found = $1 WHERE id = $2`,
		len(ratings), payload.JobID); err != nil {
		log.Printf("[scrape] job=%s failed to update films_found: %v", payload.JobID, err)
	}

	var enqueued, skipped int
	for _, r := range ratings {
		if r.TMDBMovieID == 0 {
			skipped++
			log.Printf("[scrape] job=%s skip film=%q — no TMDB ID", payload.JobID, r.Title)
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

		enrichTask, err := NewEnrichFilmTask(filmID, r.TMDBMovieID, slug, payload.JobID)
		if err != nil {
			log.Printf("[scrape] job=%s failed to create enrich task for film=%q: %v", payload.JobID, r.Title, err)
		} else {
			if _, err = w.asynqClient.Enqueue(enrichTask, asynq.TaskID("enrich:"+filmID)); err != nil && err != asynq.ErrTaskIDConflict {
				log.Printf("[scrape] job=%s failed to enqueue enrich task for film=%q: %v", payload.JobID, r.Title, err)
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
		enqueued++
	}

	log.Printf("[scrape] job=%s done: %d films queued for enrichment, %d skipped (no TMDB ID), scrape elapsed=%s",
		payload.JobID, enqueued, skipped, time.Since(scrapeStart).Round(time.Millisecond))

	// films_processed = enrich tasks queued (not total ratings) so handleEnrichFilm
	// can detect "all done" by comparing films_enriched to films_processed.
	if _, err = w.pool.Exec(ctx,
		`UPDATE scrape_jobs SET status = 'enriching', films_processed = $1, completed_at = now() WHERE id = $2`,
		enqueued, payload.JobID); err != nil {
		log.Printf("[scrape] job=%s failed to update status to enriching: %v", payload.JobID, err)
	}

	return nil
}

// reportEnrichComplete increments films_enriched for the job and logs a summary
// when all enrich tasks have finished. Call it at every exit path in handleEnrichFilm.
func (w *Workers) reportEnrichComplete(jobID string) {
	if jobID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var enriched, target int
	var startedAt *time.Time
	err := w.pool.QueryRow(ctx,
		`UPDATE scrape_jobs
		 SET films_enriched        = films_enriched + 1,
		     enrichment_started_at = COALESCE(enrichment_started_at, now()),
		     status                = CASE WHEN films_enriched + 1 >= films_processed THEN 'embedding' ELSE status END
		 WHERE id = $1
		 RETURNING films_enriched, films_processed, started_at`,
		jobID).Scan(&enriched, &target, &startedAt)
	if err != nil {
		log.Printf("[pipeline] job=%s failed to update enrichment counter: %v", jobID, err)
		return
	}

	pct := 0
	if target > 0 {
		pct = enriched * 100 / target
	}
	log.Printf("[pipeline] job=%s progress %d/%d (%d%%)", jobID, enriched, target, pct)

	if target > 0 && enriched >= target {
		elapsed := "unknown"
		if startedAt != nil {
			elapsed = time.Since(*startedAt).Round(time.Second).String()
		}
		log.Printf("[pipeline] job=%s ALL ENRICHMENT COMPLETE — %d films, total elapsed=%s",
			jobID, enriched, elapsed)
	}
}

func (w *Workers) handleEnrichFilm(ctx context.Context, t *asynq.Task) error {
	var payload models.EnrichFilmPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	enrichStart := time.Now()
	job := payload.JobID // shorter alias for log lines

	// Skip films that are already fully enriched — avoids hitting TMDB on re-runs.
	// embedding_computed is set true by handleEmbedFilm (including the title+year fallback),
	// so it's a reliable terminal signal even for films TMDB returns sparse data for.
	var alreadyEnriched bool
	if err := w.pool.QueryRow(ctx,
		`SELECT embedding_computed FROM films WHERE id = $1`,
		payload.FilmID,
	).Scan(&alreadyEnriched); err == nil && alreadyEnriched {
		log.Printf("[enrich] job=%s film=%s tmdbID=%d already enriched — skipping", job, payload.FilmID, payload.TMDBMovieID)
		defer w.reportEnrichComplete(job)
		return nil
	}

	// Serialize TMDB requests — CloudFront resets concurrent connections from the same IP.
	// The enrichment client also has a token-bucket limiter that spaces out requests.
	w.tmdbSem <- struct{}{}
	log.Printf("[enrich] job=%s film=%s tmdbID=%d waiting acquired", job, payload.FilmID, payload.TMDBMovieID)

	var filmData *enrichment.FilmData
	var err error
	var attempts int
	const maxAttempts = 8
	for attempt := 0; attempt < maxAttempts; attempt++ {
		attempts = attempt + 1
		filmData, err = w.enricher.GetFilmDetails(ctx, payload.TMDBMovieID)
		if err == nil {
			break
		}
		var rl *enrichment.RateLimitError
		if errors.As(err, &rl) {
			log.Printf("[enrich] job=%s film=%s tmdbID=%d rate-limited — waiting %s (attempt %d/%d)",
				job, payload.FilmID, payload.TMDBMovieID, rl.RetryAfter, attempts, maxAttempts)
			select {
			case <-time.After(rl.RetryAfter):
			case <-ctx.Done():
				<-w.tmdbSem
				return ctx.Err()
			}
			continue
		}
		log.Printf("[enrich] job=%s film=%s tmdbID=%d attempt %d/%d failed: %v",
			job, payload.FilmID, payload.TMDBMovieID, attempts, maxAttempts, err)
		// exponential backoff for non-rate-limit errors: 1s, 2s, 4s …
		select {
		case <-time.After(time.Duration(1<<uint(attempt)) * time.Second):
		case <-ctx.Done():
			<-w.tmdbSem
			return ctx.Err()
		}
	}
	<-w.tmdbSem

	elapsed := time.Since(enrichStart).Round(time.Millisecond)

	if err != nil {
		if strings.Contains(err.Error(), "not found on TMDB") {
			log.Printf("[enrich] job=%s film=%s tmdbID=%d SKIPPED not found on TMDB elapsed=%s",
				job, payload.FilmID, payload.TMDBMovieID, elapsed)
			_, _ = w.pool.Exec(ctx,
				`UPDATE films SET embedding_computed = true WHERE id = $1`,
				payload.FilmID)
			w.reportEnrichComplete(job)
			return nil
		}
		// Network failure after all retries — proceed with partial data so the pipeline isn't blocked.
		log.Printf("[enrich] job=%s film=%s tmdbID=%d FAILED after %d attempts elapsed=%s err=%v",
			job, payload.FilmID, payload.TMDBMovieID, attempts, elapsed, err)
		if embedTask, err2 := NewEmbedFilmTask(payload.FilmID); err2 == nil {
			if _, err2 = w.asynqClient.Enqueue(embedTask, asynq.TaskID("embed:"+payload.FilmID)); err2 != nil && err2 != asynq.ErrTaskIDConflict {
				log.Printf("[enrich] job=%s failed to enqueue embed task for film=%s: %v", job, payload.FilmID, err2)
			}
		}
		w.reportEnrichComplete(job)
		return nil
	}

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
		log.Printf("[enrich] job=%s film=%s tmdbID=%d DB update failed: %v", job, payload.FilmID, payload.TMDBMovieID, err)
		w.reportEnrichComplete(job)
		return fmt.Errorf("update film %s: %w", payload.FilmID, err)
	}

	log.Printf("[enrich] job=%s film=%s tmdbID=%d OK attempts=%d elapsed=%s directors=%v genres=%v keywords=%d",
		job, payload.FilmID, payload.TMDBMovieID, attempts, elapsed,
		filmData.Directors, filmData.Genres, len(filmData.Keywords))

	w.reportEnrichComplete(job)

	embedTask, err := NewEmbedFilmTask(payload.FilmID)
	if err != nil {
		log.Printf("[enrich] job=%s failed to create embed task for film=%s: %v", job, payload.FilmID, err)
	} else {
		if _, err = w.asynqClient.Enqueue(embedTask, asynq.TaskID("embed:"+payload.FilmID)); err != nil && err != asynq.ErrTaskIDConflict {
			log.Printf("[enrich] job=%s failed to enqueue embed task for film=%s: %v", job, payload.FilmID, err)
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
		`SELECT ur.rating, f.embedding::text, f.genres, f.directors
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
		genres    []string
		directors []string
	}

	var entries []entry
	var ratingSum float64
	var ratedCount int

	for rows.Next() {
		var rating *float64
		var embStr string
		var genres, directors []string
		if err := rows.Scan(&rating, &embStr, &genres, &directors); err != nil {
			return fmt.Errorf("scan row: %w", err)
		}
		emb, err := parseVector(embStr)
		if err != nil {
			return fmt.Errorf("parse embedding: %w", err)
		}
		entries = append(entries, entry{rating: rating, embedding: emb, genres: genres, directors: directors})
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

	// Score genres and directors: weight = max(0, rating − avg) + 1 so every
	// film contributes a base count and highly-rated films contribute more.
	genreScores := map[string]float64{}
	directorScores := map[string]float64{}
	for i, e := range entries {
		w := 1.0
		if e.rating != nil {
			w += math.Max(0, *e.rating-avgRating)
		}
		for _, g := range e.genres {
			genreScores[g] += w
		}
		for _, d := range e.directors {
			directorScores[d] += w
		}
		_ = i
	}
	topGenres := topN(genreScores, 5)
	topDirectors := topN(directorScores, 5)

	_, err = w.pool.Exec(ctx,
		`INSERT INTO taste_vectors (user_id, vector, top_genres, top_directors)
		 VALUES ($1, $2::vector, $3, $4)
		 ON CONFLICT (user_id) DO UPDATE
		   SET vector        = EXCLUDED.vector,
		       top_genres    = EXCLUDED.top_genres,
		       top_directors = EXCLUDED.top_directors,
		       computed_at   = now()`,
		payload.UserID, vecStr, topGenres, topDirectors)
	if err != nil {
		return fmt.Errorf("upsert taste vector for user=%s: %w", payload.UserID, err)
	}

	log.Printf("[vector] completed user=%s genres=%v directors=%v", payload.UserID, topGenres, topDirectors)

	// Flip the scrape job to 'complete' now that the taste vector exists.
	// The job's status was 'embedding' — this is the signal the frontend polls for.
	if _, err = w.pool.Exec(ctx,
		`UPDATE scrape_jobs SET status = 'complete' WHERE user_id = $1 AND status = 'embedding'`,
		payload.UserID); err != nil {
		log.Printf("[vector] failed to mark job complete for user=%s: %v", payload.UserID, err)
	}

	return nil
}

// topN returns the top n keys from a score map, sorted descending.
func topN(scores map[string]float64, n int) []string {
	type kv struct {
		key   string
		score float64
	}
	ranked := make([]kv, 0, len(scores))
	for k, v := range scores {
		ranked = append(ranked, kv{k, v})
	}
	// simple insertion sort — maps are small (< 50 genres)
	for i := 1; i < len(ranked); i++ {
		for j := i; j > 0 && ranked[j].score > ranked[j-1].score; j-- {
			ranked[j], ranked[j-1] = ranked[j-1], ranked[j]
		}
	}
	if n > len(ranked) {
		n = len(ranked)
	}
	out := make([]string, n)
	for i := range out {
		out[i] = ranked[i].key
	}
	return out
}

// handleGenerateBlurbs generates "why you'd click" copy for each match pair
// using Claude Haiku and stores the result in matches.why_text.
func (w *Workers) handleGenerateBlurbs(ctx context.Context, t *asynq.Task) error {
	var payload models.GenerateBlurbsPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	log.Printf("[blurbs] generating for user=%s candidates=%d", payload.UserID, len(payload.CandidateIDs))

	// Fetch taste snapshot for the requesting user.
	snapshotA, err := w.fetchTasteSnapshot(ctx, payload.UserID)
	if err != nil {
		return fmt.Errorf("fetch snapshot for user=%s: %w", payload.UserID, err)
	}

	for _, candidateID := range payload.CandidateIDs {
		snapshotB, err := w.fetchTasteSnapshot(ctx, candidateID)
		if err != nil {
			log.Printf("[blurbs] skip candidate=%s err=%v", candidateID, err)
			continue
		}

		blurb, err := llm.GenerateBlurb(ctx, w.anthropicKey, snapshotA, snapshotB)
		if err != nil {
			log.Printf("[blurbs] generation failed for pair (%s, %s) err=%v", payload.UserID, candidateID, err)
			continue
		}

		// Canonical ordering matches the unique constraint on matches(user_a, user_b).
		userA, userB := payload.UserID, candidateID
		if userA > userB {
			userA, userB = userB, userA
		}

		if _, err = w.pool.Exec(ctx,
			`UPDATE matches SET why_text = $1 WHERE user_a = $2 AND user_b = $3`,
			blurb, userA, userB); err != nil {
			log.Printf("[blurbs] failed to store blurb for pair (%s, %s) err=%v", userA, userB, err)
		}
	}

	log.Printf("[blurbs] completed user=%s", payload.UserID)
	return nil
}

// fetchTasteSnapshot loads top_genres, top_directors, and the portrait summary line for a user.
func (w *Workers) fetchTasteSnapshot(ctx context.Context, userID string) (llm.TasteSnapshot, error) {
	var snap llm.TasteSnapshot
	if err := w.pool.QueryRow(ctx,
		`SELECT top_genres, top_directors FROM taste_vectors WHERE user_id = $1`,
		userID).Scan(&snap.TopGenres, &snap.TopDirectors); err != nil {
		return snap, err
	}
	// Portrait summary line is optional — ignore if not generated yet.
	_ = w.pool.QueryRow(ctx,
		`SELECT summary_line FROM soul_portraits WHERE user_id = $1`,
		userID).Scan(&snap.SummaryLine)
	return snap, nil
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

	var synopsis, title string
	var year int
	var genres, directors, themes []string

	err := w.pool.QueryRow(ctx,
		`SELECT synopsis, genres, directors, themes, title, year FROM films
			WHERE id = $1`,
		payload.FilmID).Scan(&synopsis, &genres, &directors, &themes, &title, &year)
	if err != nil {
		return fmt.Errorf("fetch film id: %w", err)
	}

	embeddingText := strings.Join(genres, " ") + ". " +
		strings.Join(directors, " ") + ". " +
		strings.Join(themes, " ") + ". " +
		synopsis
	if strings.TrimSpace(strings.ReplaceAll(embeddingText, ".", "")) == "" {
		// Enrichment failed for this film — fall back to title+year so the
		// embedding is still meaningful and the vector computation can proceed.
		embeddingText = fmt.Sprintf("%s %d", title, year)
		log.Printf("[embed] film=%s using title+year fallback (no enrichment data)", payload.FilmID)
	}

	endpoint := w.mlServiceURL + "/embed"

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
