package handlers

import (
	"log"
	"time"

	"github.com/framed-app/api/internal/jobs"
	"github.com/framed-app/api/pkg/db"
	"github.com/jackc/pgx/v5"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
)

type Handlers struct {
	pool        *db.Pool
	asynqClient *asynq.Client
}

type OnboardRequest struct {
	LetterboxdHandle string `json:"letterboxd_handle"`
}

type OnboardResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

type StatusResponse struct {
	ID             string     `json:"id"`
	Status         string     `json:"status"`
	FilmsFound     int        `json:"films_found"`
	FilmsProcessed int        `json:"films_processed"`
	StartedAt      *time.Time `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

func New(pool *db.Pool, asynqClient *asynq.Client) *Handlers {
	return &Handlers{
		pool:        pool,
		asynqClient: asynqClient}

}

func (h *Handlers) Onboard(c *fiber.Ctx) error {
	var req OnboardRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Insert job row first — we need the DB-generated ID to return to the client
	// and to correlate with the Asynq task in the worker
	var jobID string
	err := h.pool.QueryRow(c.Context(),
		`INSERT INTO scrape_jobs (letterboxd_handle, status) VALUES ($1, 'queued') RETURNING id`,
		req.LetterboxdHandle,
	).Scan(&jobID)
	if err != nil {
		log.Printf("failed to insert scrape job: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create job",
		})
	}

	// Enqueue the Asynq task with the DB job ID so the worker can update the same row
	task, err := jobs.NewScrapeProfileTask(jobID, req.LetterboxdHandle)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create task",
		})
	}

	_, err = h.asynqClient.Enqueue(task)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to enqueue task",
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(OnboardResponse{
		JobID:  jobID,
		Status: "queued",
	})
}

func (h *Handlers) Status(c *fiber.Ctx) error {
	id := c.Params("jobId")

	var job StatusResponse
	err := h.pool.QueryRow(c.Context(), `
    SELECT id, status, films_found, films_processed, started_at, completed_at
    FROM scrape_jobs WHERE id = $1
`, id).Scan(&job.ID, &job.Status, &job.FilmsFound, &job.FilmsProcessed, &job.StartedAt, &job.CompletedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "job not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch job"})
	}

	return c.Status(fiber.StatusOK).JSON(job)
}
