package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is a thin wrapper around pgxpool so we have one place to configure
// connection behaviour across the whole service.
type Pool struct {
	*pgxpool.Pool
}

// Connect opens a connection pool to Postgres and verifies pgvector is installed.
// Returns an error rather than panicking — the caller (main.go) decides how to handle it.
func Connect(ctx context.Context, databaseURL string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	// connection pool tuning
	// these numbers are conservative for an indie app on a small instance
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 1 * time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// verify the connection is actually alive
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// verify pgvector extension is installed
	// this fails loudly at startup rather than cryptically when we first
	// try to do a vector similarity query
	var extensionExists bool
	err = pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')",
	).Scan(&extensionExists)
	if err != nil {
		return nil, fmt.Errorf("check pgvector extension: %w", err)
	}
	if !extensionExists {
		return nil, fmt.Errorf("pgvector extension is not installed — run: CREATE EXTENSION vector")
	}

	return &Pool{pool}, nil
}
