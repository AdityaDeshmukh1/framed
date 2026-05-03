package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/framed-app/api/internal/enrichment"
	"github.com/framed-app/api/internal/jobs"
	"github.com/framed-app/api/internal/scraper"
	"github.com/framed-app/api/pkg/config"
	"github.com/framed-app/api/pkg/db"
	"github.com/hibiken/asynq"
)

func main() {
	// ── Config ────────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("starting framed API [env=%s]", cfg.Env)

	// ── Context ───────────────────────────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Database ──────────────────────────────────────────────────────────────
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Printf("connected to postgres [pgvector=ready]")

	// ── Redis (Asynq) ─────────────────────────────────────────────────────────
	redisOpt := asynq.RedisClientOpt{Addr: cfg.RedisURL}

	// client enqueues jobs from HTTP handlers
	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()

	// ── Services ──────────────────────────────────────────────────────────────
	scraperSvc := scraper.New(cfg.ScraperRequestDelayMS, cfg.ScraperMaxConcurrent)
	enricherSvc := enrichment.New(cfg.TMDBApiKey)

	// ── Workers ───────────────────────────────────────────────────────────────
	workers := jobs.NewWorkers(scraperSvc, enricherSvc)

	asynqServer := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency:    10,
		RetryDelayFunc: asynq.DefaultRetryDelayFunc,
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
	})

	mux := asynq.NewServeMux()
	workers.Register(mux)

	go func() {
		log.Printf("starting job worker [concurrency=10]")
		if err := asynqServer.Run(mux); err != nil {
			log.Fatalf("job worker failed: %v", err)
		}
	}()

	// ── HTTP Server ───────────────────────────────────────────────────────────
	// TODO: initialise Fiber, register routes
	_ = pool
	_ = asynqClient
	addr := fmt.Sprintf(":%s", cfg.APIPort)
	log.Printf("API listening on %s", addr)

	// ── Graceful Shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutdown signal received — draining...")
	asynqServer.Shutdown()
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = shutdownCtx

	log.Println("framed API stopped")
}
