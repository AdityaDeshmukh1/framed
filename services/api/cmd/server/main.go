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
	"github.com/framed-app/api/internal/handlers"
	"github.com/framed-app/api/internal/jobs"
	"github.com/framed-app/api/internal/middleware"
	"github.com/framed-app/api/internal/scraper"
	"github.com/framed-app/api/pkg/config"
	"github.com/framed-app/api/pkg/db"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
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

	// ── Browser install (idempotent, ~170MB first run) ────────────────────────
	log.Printf("checking Playwright browser install...")
	if err := scraper.InstallBrowser(); err != nil {
		log.Fatalf("playwright browser install: %v", err)
	}

	// ── Services ──────────────────────────────────────────────────────────────
	scraperSvc := scraper.NewPlaywrightScraper()
	enricherSvc := enrichment.New(cfg.TMDBApiKey)

	// ── Workers ───────────────────────────────────────────────────────────────
	workers := jobs.NewWorkers(scraperSvc, enricherSvc, pool, asynqClient, cfg.AnthropicApiKey, cfg.MLServiceURL)
	log.Printf("ML service URL: %s", cfg.MLServiceURL)

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
	app := fiber.New()
	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${method} ${path} → ${status} (${latency})\n",
		TimeFormat: "15:04:05",
	}))
	h := handlers.New(pool, asynqClient, cfg.AnthropicApiKey)
	auth := middleware.ClerkAuth(pool, cfg.ClerkSecretKey)

	// All routes require a valid Clerk JWT.
	app.Post("/onboard", auth, h.Onboard)
	app.Get("/status/:jobId", auth, h.Status)

	app.Post("/match", auth, h.Match)
	app.Get("/matches", auth, h.GetMatches)
	app.Get("/match/:matchId", auth, h.GetMatch)

	app.Post("/soul", auth, h.Soul)
	app.Get("/soul", auth, h.GetSoul)

	addr := fmt.Sprintf(":%s", cfg.APIPort)
	go app.Listen(addr)
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
