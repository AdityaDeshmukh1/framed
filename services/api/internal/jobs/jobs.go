package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/framed-app/api/pkg/models"
	"github.com/hibiken/asynq"
)

func NewScrapeProfileTask(userID, handle string) (*asynq.Task, error) {
	payload := models.ScrapeProfilePayload{
		UserID:           userID,
		LetterboxdHandle: handle,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	return asynq.NewTask(string(models.JobTypeScrapeProfile), bytes), nil
}

func NewComputeVectorTask(userID string) (*asynq.Task, error) {
	payload := models.ComputeVectorPayload{
		UserID: userID,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	return asynq.NewTask(string(models.JobTypeComputeVector), bytes), nil
}
