package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/framed-app/api/internal/embedding"
	"github.com/framed-app/api/internal/enrichment"
	"github.com/framed-app/api/internal/importer"
	"github.com/framed-app/api/internal/llm"
	"github.com/framed-app/api/internal/taste"
	"github.com/framed-app/api/pkg/db"
	"github.com/framed-app/api/pkg/models"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
)

// Workers holds all dependencies the job handlers need.
type Workers struct {
	importers    *importer.Registry
	asynqClient  *asynq.Client
	enricher     *enrichment.TMDBClient
	pool         *db.Pool
	anthropicKey string
	embedder     *embedding.Client
	// tmdbSem caps concurrent TMDB requests — CloudFront resets connections
	// when too many hit simultaneously from the same IP.
	tmdbSem chan struct{}
}

func NewWorkers(importers *importer.Registry, enricher *enrichment.TMDBClient, pool *db.Pool, asynqClient *asynq.Client, anthropicKey, mlServiceURL string) *Workers {
	return &Workers{
		importers:    importers,
		enricher:     enricher,
		pool:         pool,
		asynqClient:  asynqClient,
		anthropicKey: anthropicKey,
		embedder:     embedding.NewClient(mlServiceURL),
		tmdbSem:      make(chan struct{}, 5),
	}
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

	imp, err := w.importers.Get(importer.Source(payload.Source))
	if err != nil {
		// Retrying won't enable a source — fail the job now.
		return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
	}
	req := importer.Request{Handle: payload.LetterboxdHandle}
	for _, r := range payload.Ratings {
		req.Submitted = append(req.Submitted, importer.Submitted{TMDBMovieID: r.TMDBMovieID, Rating: r.Rating, Liked: r.Liked})
	}
	ratings, err := imp.Import(ctx, req)
	if err != nil {
		return fmt.Errorf("import from %s: %w", imp.Source(), err)
	}

	log.Printf("[scrape] job=%s imported %d films from %s in %s", payload.JobID, len(ratings), imp.Source(), time.Since(scrapeStart).Round(time.Millisecond))

	// Update films_found count now that we know the total
	if _, err := w.pool.Exec(ctx,
		`UPDATE scrape_jobs SET films_found = $1 WHERE id = $2`,
		len(ratings), payload.JobID); err != nil {
		log.Printf("[scrape] job=%s failed to update films_found: %v", payload.JobID, err)
	}

	var enqueued, ready, skipped int
	for _, r := range ratings {
		// Sources that don't know the TMDB ID leave it 0; resolve it via TMDB search.
		if r.TMDBMovieID == 0 && r.Title != "" {
			id, err := w.enricher.SearchFilm(ctx, r.Title, r.Year)
			if err != nil {
				log.Printf("[scrape] job=%s TMDB search failed film=%q year=%d: %v", payload.JobID, r.Title, r.Year, err)
			} else {
				r.TMDBMovieID = id
			}
		}
		if r.TMDBMovieID == 0 {
			skipped++
			log.Printf("[scrape] job=%s skip film=%q year=%d — no TMDB ID", payload.JobID, r.Title, r.Year)
			continue
		}

		slug := r.LetterboxdSlug
		var slugArg *string // NULL when the source doesn't know the slug
		if slug != "" {
			slugArg = &slug
		}

		// Write or Fetch film from the films table
		var filmID string
		err = w.pool.QueryRow(ctx,
			`INSERT INTO films (tmdb_id, letterboxd_slug, title, year)
VALUES ($1, $2, $3, $4)
ON CONFLICT (tmdb_id) DO NOTHING
RETURNING id`,
			r.TMDBMovieID, slugArg, r.Title, r.Year).Scan(&filmID)

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

		// Store (or update, on a re-import or a retaken quiz) the rating.
		if _, err = w.pool.Exec(ctx,
			`INSERT INTO user_ratings (user_id, film_id, rating, watched_date, rewatch, liked)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (user_id, film_id) DO UPDATE SET
			   rating = EXCLUDED.rating, liked = EXCLUDED.liked, rewatch = EXCLUDED.rewatch,
			   watched_date = COALESCE(EXCLUDED.watched_date, user_ratings.watched_date)`,
			payload.UserID, filmID, r.Rating, r.WatchedDate, r.Rewatch, r.Liked,
		); err != nil {
			return fmt.Errorf("upsert rating: %w", err)
		}

		// Catalogue films are already enriched and embedded at the current
		// text version: nothing to fetch.
		var isReady bool
		if err := w.pool.QueryRow(ctx,
			`SELECT embedding_computed AND embedding IS NOT NULL AND embedding_version >= $2 FROM films WHERE id = $1`,
			filmID, embedding.TextVersion).Scan(&isReady); err != nil {
			return fmt.Errorf("check film %s: %w", filmID, err)
		}
		if isReady {
			ready++
			continue
		}

		enrichTask, err := NewEnrichFilmTask(filmID, r.TMDBMovieID, slug, payload.JobID)
		if err != nil {
			log.Printf("[scrape] job=%s failed to create enrich task for film=%q: %v", payload.JobID, r.Title, err)
		} else {
			if _, err = w.asynqClient.Enqueue(enrichTask, asynq.TaskID("enrich:"+filmID)); err != nil && err != asynq.ErrTaskIDConflict {
				log.Printf("[scrape] job=%s failed to enqueue enrich task for film=%q: %v", payload.JobID, r.Title, err)
			}
		}
		enqueued++
	}

	log.Printf("[scrape] job=%s done: %d films queued for enrichment, %d already in catalogue, %d skipped (no TMDB ID), scrape elapsed=%s",
		payload.JobID, enqueued, ready, skipped, time.Since(scrapeStart).Round(time.Millisecond))

	// Everything was already in the catalogue (always true for the quiz):
	// no enrich/embed task will fire, so build the taste vector now.
	if enqueued == 0 {
		if ready == 0 {
			return fmt.Errorf("job=%s: no usable films: %w", payload.JobID, asynq.SkipRetry)
		}
		if _, err := w.pool.Exec(ctx,
			`UPDATE scrape_jobs SET status = 'embedding', films_processed = 0, completed_at = now() WHERE id = $1`,
			payload.JobID); err != nil {
			log.Printf("[scrape] job=%s failed to update status to embedding: %v", payload.JobID, err)
		}
		task, err := NewComputeVectorTask(payload.UserID)
		if err != nil {
			return err
		}
		if _, err := w.asynqClient.Enqueue(task); err != nil {
			return fmt.Errorf("enqueue taste vector: %w", err)
		}
		return nil
	}

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
		if err == nil || errors.Is(err, enrichment.ErrNotFound) {
			break // success, or a permanent miss that retrying can't fix
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
		if errors.Is(err, enrichment.ErrNotFound) {
			log.Printf("[enrich] job=%s film=%s tmdbID=%d SKIPPED not found on TMDB elapsed=%s",
				job, payload.FilmID, payload.TMDBMovieID, elapsed)
			_, _ = w.pool.Exec(ctx,
				`UPDATE films SET embedding_computed = true WHERE id = $1`,
				payload.FilmID)
			w.reportEnrichComplete(job)
			w.enqueueReadyVectors(ctx, payload.FilmID)
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
         SET synopsis = $1, directors = $2, genres = $3, poster_path = $4, themes = $5,
             language = $6, country = $7, runtime = $8, popularity = $9
         WHERE id = $10`,
		filmData.Synopsis,
		filmData.Directors,
		filmData.Genres,
		filmData.PosterPath,
		filmData.Keywords,
		nullIfEmpty(filmData.Language),
		nullIfEmpty(filmData.Country),
		filmData.Runtime,
		filmData.Popularity,
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
		`SELECT ur.rating, ur.liked, f.embedding::text, f.genres, f.directors
		 FROM user_ratings ur
		 JOIN films f ON f.id = ur.film_id
		 WHERE ur.user_id = $1 AND f.embedding IS NOT NULL`,
		payload.UserID)
	if err != nil {
		return fmt.Errorf("fetch ratings: %w", err)
	}
	defer rows.Close()

	type entry struct {
		taste.Entry
		genres    []string
		directors []string
	}

	var entries []entry
	var ratingSum float64
	var ratedCount int

	for rows.Next() {
		var rating *float64
		var liked bool
		var embStr string
		var genres, directors []string
		if err := rows.Scan(&rating, &liked, &embStr, &genres, &directors); err != nil {
			return fmt.Errorf("scan row: %w", err)
		}
		emb, err := embedding.ParseVector(embStr)
		if err != nil {
			return fmt.Errorf("parse embedding: %w", err)
		}
		entries = append(entries, entry{
			Entry:     taste.Entry{Rating: rating, Liked: liked, Embedding: emb},
			genres:    genres,
			directors: directors,
		})
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

	// No centring yet: with only a handful of users the catalogue mean is just
	// the midpoint between them, which pushes every pair to opposite sides
	// (measured cos ≈ -0.78 for different users). Matching rescales raw cosine
	// instead — see matching.displayScore. Revisit with a stable reference mean.
	tasteEntries := make([]taste.Entry, len(entries))
	for i, e := range entries {
		tasteEntries[i] = e.Entry
	}
	center, err := w.catalogueMean(ctx)
	if err != nil {
		return fmt.Errorf("load catalogue mean: %w", err)
	}
	opts := taste.Production(center)
	if center == nil {
		log.Printf("[vector] WARNING: no catalogue_mean in taste_reference (run cmd/catalogue); using V1 weights for user=%s", payload.UserID)
		opts = taste.V1
	}
	vec := taste.Vector(tasteEntries, opts)
	if vec == nil {
		return fmt.Errorf("taste vector for user=%s has zero length", payload.UserID)
	}

	vecStr := embedding.FormatVector(vec)

	// Score genres and directors: weight = max(0, rating − avg) + 1 so every
	// film contributes a base count and highly-rated films contribute more.
	genreScores := map[string]float64{}
	directorScores := map[string]float64{}
	for _, e := range entries {
		w := 1.0
		if e.Rating != nil {
			w += math.Max(0, *e.Rating-avgRating)
		}
		for _, g := range e.genres {
			genreScores[g] += w
		}
		for _, d := range e.directors {
			directorScores[d] += w
		}
	}
	topGenres := topN(genreScores, 5)
	topDirectors := topN(directorScores, 5)

	_, err = w.pool.Exec(ctx,
		`INSERT INTO taste_vectors (user_id, vector, top_genres, top_directors, film_count, confidence)
		 VALUES ($1, $2::vector, $3, $4, $5, $6)
		 ON CONFLICT (user_id) DO UPDATE
		   SET vector        = EXCLUDED.vector,
		       top_genres    = EXCLUDED.top_genres,
		       top_directors = EXCLUDED.top_directors,
		       film_count    = EXCLUDED.film_count,
		       confidence    = EXCLUDED.confidence,
		       computed_at   = now()`,
		payload.UserID, vecStr, topGenres, topDirectors, len(entries), taste.Confidence(ratedCount))
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

func (w *Workers) handleEmbedFilm(ctx context.Context, t *asynq.Task) error {
	var payload models.EmbedFilmPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	log.Printf("[embed] starting embed for film=%s", payload.FilmID)

	var f embedding.Film
	var language, country *string
	err := w.pool.QueryRow(ctx,
		`SELECT synopsis, genres, directors, themes, title, year, language, country
		 FROM films WHERE id = $1`,
		payload.FilmID).Scan(&f.Synopsis, &f.Genres, &f.Directors, &f.Keywords, &f.Title, &f.Year, &language, &country)
	if err != nil {
		return fmt.Errorf("fetch film id: %w", err)
	}
	if language != nil {
		f.Language = *language
	}
	if country != nil {
		f.Country = *country
	}

	vec, err := w.embedder.Embed(ctx, embedding.Text(f, embedding.TextVersion))
	if err != nil {
		return fmt.Errorf("film=%s: %w", payload.FilmID, err)
	}

	_, err = w.pool.Exec(ctx,
		`UPDATE films
		 SET embedding = $1::vector, embedding_computed = true, embedding_version = $2
		 WHERE id = $3`,
		embedding.FormatVector(vec), embedding.TextVersion, payload.FilmID)
	if err != nil {
		return fmt.Errorf("update film %s: %w", payload.FilmID, err)
	}

	log.Printf("[embed] completed film=%s", payload.FilmID)

	w.enqueueReadyVectors(ctx, payload.FilmID)
	return nil
}

// enqueueReadyVectors enqueues ComputeVector for every user who rated filmID
// and has no films left waiting for an embedding. Call it whenever a film
// reaches a terminal state — embedded, or skipped as not found on TMDB —
// otherwise a user whose last pending film was skipped would never finish.
func (w *Workers) enqueueReadyVectors(ctx context.Context, filmID string) {
	userRows, err := w.pool.Query(ctx,
		`SELECT DISTINCT ur.user_id FROM user_ratings ur WHERE ur.film_id = $1 AND ur.user_id IS NOT NULL`,
		filmID)
	if err != nil {
		log.Printf("[embed] failed to query users for film=%s err=%v", filmID, err)
		return
	}
	var userIDs []string
	for userRows.Next() {
		var userID string
		if err := userRows.Scan(&userID); err == nil {
			userIDs = append(userIDs, userID)
		}
	}
	userRows.Close()

	for _, userID := range userIDs {
		// embedding_computed is the terminal flag: true once a film is embedded
		// or permanently skipped, so skipped films don't block the vector.
		var pending int
		if err := w.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM user_ratings ur
			 JOIN films f ON f.id = ur.film_id
			 WHERE ur.user_id = $1 AND f.tmdb_id != 0 AND NOT f.embedding_computed`,
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
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// catalogueMean loads the centring vector written by cmd/catalogue; nil if
// it hasn't been computed yet.
func (w *Workers) catalogueMean(ctx context.Context) ([]float32, error) {
	var s string
	err := w.pool.QueryRow(ctx, `SELECT vector::text FROM taste_reference WHERE name = 'catalogue_mean'`).Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return embedding.ParseVector(s)
}
