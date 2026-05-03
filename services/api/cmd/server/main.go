package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/framed-app/api/pkg/config"
	"github.com/framed-app/api/pkg/db"
)

func main() {
	// ── Config ────────────────────────────────────────────────────────────────
	// Load first. If any required env var is missing, we panic here
	// rather than at the point of use. Loud failure at startup > silent
	// failure in production.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("starting frame API [env=%s]", cfg.Env)

	// ── Context ───────────────────────────────────────────────────────────────
	// Root context with cancellation. We pass this down to every long-lived
	// component — database, workers, HTTP server — so that when we receive
	// a shutdown signal, everything can clean up gracefully.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Database ──────────────────────────────────────────────────────────────
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Printf("connected to postgres [pgvector=ready]")

	// ── HTTP Server ───────────────────────────────────────────────────────────
	// TODO: initialise Fiber, register routes, start listening
	// This will be built out in the next step
	_ = pool
	_ = cfg

	addr := fmt.Sprintf(":%s", cfg.APIPort)
	log.Printf("API listening on %s", addr)

	// ── Graceful Shutdown ─────────────────────────────────────────────────────
	// Block until we receive SIGINT or SIGTERM (ctrl+c or kill).
	// Then cancel the root context (signals all goroutines to stop),
	// give everything 10 seconds to finish, then exit.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutdown signal received — draining...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// TODO: server.ShutdownWithContext(shutdownCtx)
	_ = shutdownCtx

	log.Println("frame API stopped")
}
