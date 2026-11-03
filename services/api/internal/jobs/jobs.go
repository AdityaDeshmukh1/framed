package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/framed-app/api/pkg/models"
	"github.com/hibiken/asynq"
)

func NewScrapeProfileTask(jobID, userID, handle, source string) (*asynq.Task, error) {
	payload := models.ScrapeProfilePayload{
		JobID:            jobID,
		UserID:           userID,
		LetterboxdHandle: handle,
		Source:           source,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	return asynq.NewTask(string(models.JobTypeScrapeProfile), bytes, asynq.MaxRetry(3)), nil
}

func NewComputeVectorTask(userID string) (*asynq.Task, error) {
	payload := models.ComputeVectorPayload{
		UserID: userID,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	return asynq.NewTask(string(models.JobTypeComputeVector), bytes, asynq.MaxRetry(3)), nil
}

func NewEnrichFilmTask(filmID string, tmdbID int, slug, jobID string) (*asynq.Task, error) {
	payload := models.EnrichFilmPayload{
		FilmID:         filmID,
		TMDBMovieID:    tmdbID,
		LetterboxdSlug: slug,
		JobID:          jobID,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	return asynq.NewTask(string(models.JobTypeEnrichFilm), bytes), nil
}

func NewEmbedFilmTask(filmdId string) (*asynq.Task, error) {
	payload := models.EmbedFilmPayload{
		FilmID: filmdId,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	// Few retries: if the embedding service is down, fail the job within
	// minutes rather than retrying for days with the default 25.
	return asynq.NewTask(string(models.JobTypeEmbedFilm), bytes, asynq.MaxRetry(5)), nil
}

func NewGenerateBlurbsTask(userID string, candidateIDs []string) (*asynq.Task, error) {
	payload := models.GenerateBlurbsPayload{
		UserID:       userID,
		CandidateIDs: candidateIDs,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	return asynq.NewTask(string(models.JobTypeGenerateBlurbs), bytes), nil
}
