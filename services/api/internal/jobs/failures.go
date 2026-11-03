package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/framed-app/api/pkg/models"
	"github.com/hibiken/asynq"
)

// activeJobStatuses are the scrape_jobs states a pipeline can be stuck in.
const activeJobStatuses = `('queued','scraping','enriching','embedding')`

// HandleError is the Asynq ErrorHandler. Every failed attempt is logged; once a
// task has used its final retry, the scrape job it belongs to is marked
// 'failed' so the frontend stops polling a job that will never finish.
func (w *Workers) HandleError(ctx context.Context, task *asynq.Task, err error) {
	retried, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)
	log.Printf("[jobs] task=%s attempt %d/%d failed: %v", task.Type(), retried+1, maxRetry+1, err)
	if retried < maxRetry {
		return
	}

	// The task context may already be cancelled; use a fresh one for cleanup.
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch models.JobType(task.Type()) {
	case models.JobTypeScrapeProfile:
		var p models.ScrapeProfilePayload
		if json.Unmarshal(task.Payload(), &p) == nil {
			w.failJob(dbCtx, `id = $1`, p.JobID, "could not import Letterboxd profile: "+err.Error())
		}
	case models.JobTypeEmbedFilm:
		var p models.EmbedFilmPayload
		if json.Unmarshal(task.Payload(), &p) == nil {
			w.failJob(dbCtx,
				`user_id IN (SELECT user_id FROM user_ratings WHERE film_id = $1)`,
				p.FilmID, "embedding service unavailable: "+err.Error())
		}
	case models.JobTypeComputeVector:
		var p models.ComputeVectorPayload
		if json.Unmarshal(task.Payload(), &p) == nil {
			w.failJob(dbCtx, `user_id = $1`, p.UserID, "could not build taste profile: "+err.Error())
		}
	}
}

// failJob marks every active scrape job matching where (with a single $1 arg)
// as failed with the given reason.
func (w *Workers) failJob(ctx context.Context, where, arg, reason string) {
	tag, err := w.pool.Exec(ctx,
		`UPDATE scrape_jobs SET status = 'failed', error_message = $2, completed_at = now()
		 WHERE status IN `+activeJobStatuses+` AND `+where,
		arg, reason)
	if err != nil {
		log.Printf("[jobs] failed to mark job failed (%s=%s): %v", where, arg, err)
		return
	}
	if tag.RowsAffected() > 0 {
		log.Printf("[jobs] marked %d job(s) failed: %s", tag.RowsAffected(), reason)
	}
}

// CheckMLService pings the embedding service once so a missing service is
// obvious at startup instead of surfacing as a silent stall mid-pipeline.
func CheckMLService(baseURL string) error {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return fmt.Errorf("embedding service unreachable at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("embedding service at %s returned %d", baseURL, resp.StatusCode)
	}
	return nil
}
