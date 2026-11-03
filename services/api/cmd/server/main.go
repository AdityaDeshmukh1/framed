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
	"github.com/framed-app/api/internal/importer"
	"github.com/framed-app/api/internal/jobs"
	"github.com/framed-app/api/internal/middleware"
	"github.com/framed-app/api/pkg/config"
	"github.com/framed-app/api/pkg/db"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
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

	// ── Import sources ────────────────────────────────────────────────────────
	importers, err := buildImporters(cfg.ImportSources, pool)
	if err != nil {
		log.Fatalf("import sources: %v", err)
	}
	log.Printf("import sources: %v (default %s)", cfg.ImportSources, importers.Default())

	// ── Services ──────────────────────────────────────────────────────────────
	enricherSvc := enrichment.New(cfg.TMDBApiKey)

	// ── Workers ───────────────────────────────────────────────────────────────
	workers := jobs.NewWorkers(importers, enricherSvc, pool, asynqClient, cfg.AnthropicApiKey, cfg.MLServiceURL)
	log.Printf("ML service URL: %s", cfg.MLServiceURL)
	if err := jobs.CheckMLService(cfg.MLServiceURL); err != nil {
		log.Printf("WARNING: %v — onboarding will fail at the embedding step until it is running (make dev)", err)
	}

	asynqServer := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency:    10,
		RetryDelayFunc: asynq.DefaultRetryDelayFunc,
		ErrorHandler:   asynq.ErrorHandlerFunc(workers.HandleError),
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
	app.Use(compress.New()) // the quiz pool is ~1MB of JSON; gzip takes it to ~400KB
	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${method} ${path} → ${status} (${latency})\n",
		TimeFormat: "15:04:05",
	}))
	h := handlers.New(pool, asynqClient, importers, cfg.AnthropicApiKey)
	auth := middleware.ClerkAuth(pool, cfg.ClerkSecretKey)

	// All routes require a valid Clerk JWT.
	app.Post("/onboard", auth, h.Onboard)
	app.Get("/status/:jobId", auth, h.Status)

	app.Post("/match", auth, h.Match)
	app.Get("/matches", auth, h.GetMatches)
	app.Get("/match/:matchId", auth, h.GetMatch)

	app.Get("/quiz/pool", auth, h.QuizPool)
	app.Put("/me/basics", auth, h.SaveBasics)

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

// buildImporters constructs the enabled import sources in order; the first is
// the onboarding default. Chromium is only installed when letterboxd_web is on.
func buildImporters(sources []string, pool *db.Pool) (*importer.Registry, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("IMPORT_SOURCES is empty")
	}
	var imps []importer.Importer
	for _, s := range sources {
		switch importer.Source(s) {
		case importer.SourceLetterboxdRSS:
			imps = append(imps, importer.NewLetterboxdRSS())
		case importer.SourceLetterboxdWeb:
			log.Printf("checking Playwright browser install...")
			if err := importer.InstallBrowser(); err != nil {
				return nil, fmt.Errorf("playwright browser install: %w", err)
			}
			imps = append(imps, importer.NewLetterboxdWeb())
		case importer.SourceQuiz:
			imps = append(imps, importer.NewQuiz(catalogueLookup(pool)))
		default:
			return nil, fmt.Errorf("unknown import source %q", s)
		}
	}
	return importer.NewRegistry(imps...), nil
}

// catalogueLookup lets the quiz importer check answers against `films`.
func catalogueLookup(pool *db.Pool) importer.CatalogueLookup {
	return func(ctx context.Context, ids []int) (map[int]importer.CatalogueFilm, error) {
		rows, err := pool.Query(ctx, `SELECT tmdb_id, title, year FROM films WHERE tmdb_id = ANY($1) AND embedding IS NOT NULL`, ids)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := map[int]importer.CatalogueFilm{}
		for rows.Next() {
			var id int
			var f importer.CatalogueFilm
			if err := rows.Scan(&id, &f.Title, &f.Year); err != nil {
				return nil, err
			}
			out[id] = f
		}
		return out, rows.Err()
	}
}
