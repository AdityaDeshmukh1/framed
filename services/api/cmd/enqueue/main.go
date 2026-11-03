package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/framed-app/api/internal/jobs"
	"github.com/framed-app/api/pkg/models"
	"github.com/hibiken/asynq"
)

func main() {
	job := flag.String("job", "", "job type: scrape | enrich | embed | vector")
	userID := flag.String("user", "", "user UUID")
	filmID := flag.String("film", "", "film UUID (embed/enrich)")
	handle := flag.String("handle", "", "letterboxd handle (scrape)")
	source := flag.String("source", "", "import source (scrape, optional; default from server config)")
	jobID := flag.String("jobid", "", "scrape_jobs UUID to track progress against (scrape, optional)")
	ratingsJSON := flag.String("ratings", "", `quiz answers as JSON, e.g. '[{"tmdb_id":550,"rating":4.5,"liked":true}]' (scrape -source quiz)`)
	flag.Parse()

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisURL})
	defer client.Close()

	var task interface{ Type() string }
	_ = task

	switch *job {
	case "scrape":
		var ratings []models.SubmittedRating
		if *ratingsJSON != "" {
			if err := json.Unmarshal([]byte(*ratingsJSON), &ratings); err != nil {
				log.Fatalf("parse -ratings: %v", err)
			}
		}
		t, err := jobs.NewScrapeProfileTask(*jobID, *userID, *handle, *source, ratings...)
		if err != nil {
			log.Fatalf("create task: %v", err)
		}
		if _, err := client.Enqueue(t); err != nil {
			log.Fatalf("enqueue: %v", err)
		}
	case "embed":
		t, err := jobs.NewEmbedFilmTask(*filmID)
		if err != nil {
			log.Fatalf("create task: %v", err)
		}
		if _, err := client.Enqueue(t); err != nil {
			log.Fatalf("enqueue: %v", err)
		}
	case "vector":
		t, err := jobs.NewComputeVectorTask(*userID)
		if err != nil {
			log.Fatalf("create task: %v", err)
		}
		if _, err := client.Enqueue(t); err != nil {
			log.Fatalf("enqueue: %v", err)
		}
	default:
		log.Fatalf("unknown job %q — use: scrape | embed | vector", *job)
	}

	log.Printf("enqueued %s", *job)
}
