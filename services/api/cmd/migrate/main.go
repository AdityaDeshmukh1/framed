package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

const migrationsDir = "migrations"

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate <up|down|reset>")
	}

	// Load .env for local dev — silently ignored if file doesn't exist.
	// In production, env vars are injected directly.
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	switch os.Args[1] {
	case "up":
		must(runUp(ctx, conn))
	case "down":
		must(runDown(ctx, conn))
	case "reset":
		must(runReset(ctx, conn))
	default:
		log.Fatalf("unknown command %q — use: up, down, reset", os.Args[1])
	}
}

// ensureMigrationsTable creates the tracking table if it doesn't exist.
func ensureMigrationsTable(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT        PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
}

// runUp applies all pending migrations in filename order.
func runUp(ctx context.Context, conn *pgx.Conn) error {
	if err := ensureMigrationsTable(ctx, conn); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(files)

	// Build set of already-applied migrations.
	applied := map[string]bool{}
	rows, err := conn.Query(ctx, `SELECT filename FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("fetch applied migrations: %w", err)
	}
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return err
		}
		applied[f] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	var ran int
	for _, file := range files {
		name := filepath.Base(file)
		if applied[name] {
			log.Printf("  skip  %s", name)
			continue
		}

		sql, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit %s: %w", name, err)
		}

		log.Printf("  apply %s", name)
		ran++
	}

	if ran == 0 {
		log.Println("already up to date")
	} else {
		log.Printf("%d migration(s) applied", ran)
	}
	return nil
}

// runDown rolls back the most recently applied migration.
// Requires a corresponding <name>_down.sql file alongside the up migration.
func runDown(ctx context.Context, conn *pgx.Conn) error {
	if err := ensureMigrationsTable(ctx, conn); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	var last string
	err := conn.QueryRow(ctx,
		`SELECT filename FROM schema_migrations ORDER BY applied_at DESC LIMIT 1`,
	).Scan(&last)
	if err != nil {
		return fmt.Errorf("no applied migrations found")
	}

	downFile := strings.TrimSuffix(filepath.Join(migrationsDir, last), ".sql") + "_down.sql"
	sql, err := os.ReadFile(downFile)
	if err != nil {
		return fmt.Errorf("no rollback file for %s — create %s to enable down migrations", last, downFile)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("rollback %s: %w", last, err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE filename = $1`, last); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	log.Printf("rolled back %s", last)
	return nil
}

// runReset drops the public schema and re-applies all migrations from scratch.
// Refuses to run against any non-localhost database as a safety guard.
func runReset(ctx context.Context, conn *pgx.Conn) error {
	dbURL := os.Getenv("DATABASE_URL")
	if !strings.Contains(dbURL, "localhost") && !strings.Contains(dbURL, "127.0.0.1") {
		return fmt.Errorf("reset refused: DATABASE_URL does not point to localhost — use a real migration rollback in production")
	}

	log.Println("dropping public schema...")
	if _, err := conn.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		return fmt.Errorf("drop schema: %w", err)
	}
	log.Println("schema dropped — re-applying all migrations")
	return runUp(ctx, conn)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
