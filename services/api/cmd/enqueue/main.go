package main

import (
	"flag"
	"log"
	"os"

	"github.com/framed-app/api/internal/jobs"
	"github.com/hibiken/asynq"
)

func main() {
	job := flag.String("job", "", "job type: scrape | enrich | embed | vector")
	userID := flag.String("user", "", "user UUID")
	filmID := flag.String("film", "", "film UUID (embed/enrich)")
	handle := flag.String("handle", "", "letterboxd handle (scrape)")
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
		t, err := jobs.NewScrapeProfileTask(*userID, *handle)
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
