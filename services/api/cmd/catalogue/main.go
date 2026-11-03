// Command catalogue loads crowd rating stats and pre-builds the film catalogue.
//
//		go run ./cmd/catalogue -movielens ../../data/movielens/ml-32m -top 3000
//
//	 1. Streams MovieLens ratings into film_stats (mean, spread and count per
//	    film, keyed by TMDB id). MovieLens is research/non-commercial only.
//	 2. Enriches and embeds the top-N most-rated films into `films`, so most
//	    films a new user rates are already there (faster onboarding) and the
//	    quiz/evaluation have a real catalogue to draw from.
//	 3. Re-embeds any film whose embedding_version is below the current
//	    embedding.TextVersion, then queues taste-vector recomputes for users.
//
// Needs DATABASE_URL, TMDB_API_KEY (+TMDB_BASE_URL), the ML service and Redis.
package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/framed-app/api/internal/embedding"
	"github.com/framed-app/api/internal/enrichment"
	"github.com/framed-app/api/internal/jobs"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	mlDir := flag.String("movielens", "../../data/movielens/ml-32m", "MovieLens directory (links.csv, ratings.csv)")
	top := flag.Int("top", 3000, "how many of the most-rated films to pre-load into the catalogue")
	minRatings := flag.Int("min-ratings", 20, "skip films with fewer MovieLens ratings in film_stats")
	skipStats := flag.Bool("skip-stats", false, "don't reload film_stats")
	workers := flag.Int("workers", 6, "concurrent TMDB+embed workers")
	quizPool := flag.Int("quiz-pool", 400, "how many films to mark as quiz cards (0 = leave as is)")
	flag.Parse()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	embedder := embedding.NewClient(envOr("ML_SERVICE_URL", "http://localhost:8001"))
	if err := embedder.Health(); err != nil {
		log.Fatal(err)
	}

	// 1. Crowd stats.
	if !*skipStats {
		start := time.Now()
		stats, err := loadMovieLens(*mlDir)
		if err != nil {
			log.Fatal(err)
		}
		n, err := writeStats(ctx, pool, stats, *minRatings)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("[catalogue] film_stats: %d films (≥%d ratings) in %s", n, *minRatings, time.Since(start).Round(time.Second))
	}

	// 2 + 3. Which films need (re)building.
	targets, err := pendingFilms(ctx, pool, *top)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[catalogue] %d films to enrich+embed (top %d by ratings, plus anything below text v%d)", len(targets), *top, embedding.TextVersion)

	tmdb := enrichment.New(mustEnv("TMDB_API_KEY"))
	var done, failed atomic.Int64
	ch := make(chan int)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range ch {
				if err := buildFilm(ctx, pool, tmdb, embedder, id); err != nil {
					failed.Add(1)
					if !errors.Is(err, enrichment.ErrNotFound) {
						log.Printf("[catalogue] tmdb=%d: %v", id, err)
					}
				}
				if n := done.Add(1); n%250 == 0 {
					log.Printf("[catalogue] %d/%d (%s)", n, len(targets), time.Since(start).Round(time.Second))
				}
			}
		}()
	}
	for _, id := range targets {
		ch <- id
	}
	close(ch)
	wg.Wait()
	log.Printf("[catalogue] built %d films, %d failed, in %s", done.Load()-failed.Load(), failed.Load(), time.Since(start).Round(time.Second))

	if *quizPool > 0 {
		n, err := markQuizPool(ctx, pool, *quizPool)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("[catalogue] quiz pool: %d films", n)
	}

	if err := storeCatalogueMean(ctx, pool); err != nil {
		log.Fatal(err)
	}

	// Existing users' vectors were built from old embeddings.
	if err := requeueVectors(ctx, pool); err != nil {
		log.Printf("[catalogue] could not queue taste-vector recomputes: %v", err)
	}
}

/* ── MovieLens ──────────────────────────────────────────────────────────── */

type stat struct {
	tmdbID        int
	sum, sumSq, n float64
}

func loadMovieLens(dir string) (map[int]*stat, error) {
	links, err := readLinks(filepath.Join(dir, "links.csv"))
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(dir, "ratings.csv"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stats := map[int]*stat{} // by MovieLens movieId
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	sc.Scan() // header
	var rows int
	for sc.Scan() {
		// userId,movieId,rating,timestamp — split by hand; csv.Reader is ~5x slower here.
		movie, rating, ok := parseRating(sc.Bytes())
		if !ok {
			continue
		}
		tmdb, ok := links[movie]
		if !ok {
			continue
		}
		s := stats[movie]
		if s == nil {
			s = &stat{tmdbID: tmdb}
			stats[movie] = s
		}
		s.sum += rating
		s.sumSq += rating * rating
		s.n++
		if rows++; rows%5_000_000 == 0 {
			log.Printf("[catalogue] read %dM ratings", rows/1_000_000)
		}
	}
	return stats, sc.Err()
}

func parseRating(line []byte) (movie int, rating float64, ok bool) {
	var fields [3][]byte
	f, startIdx := 0, 0
	for i, c := range line {
		if c == ',' {
			if f < 3 {
				fields[f] = line[startIdx:i]
			}
			f++
			startIdx = i + 1
		}
	}
	if f < 3 {
		return 0, 0, false
	}
	m, err1 := strconv.Atoi(string(fields[1]))
	r, err2 := strconv.ParseFloat(string(fields[2]), 64)
	return m, r, err1 == nil && err2 == nil
}

func readLinks(path string) (map[int]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	if _, err := r.Read(); err != nil {
		return nil, err
	}
	links := map[int]int{}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		movie, err1 := strconv.Atoi(rec[0])
		tmdb, err2 := strconv.Atoi(rec[2])
		if err1 == nil && err2 == nil && tmdb > 0 {
			links[movie] = tmdb
		}
	}
	return links, nil
}

func writeStats(ctx context.Context, pool *pgxpool.Pool, stats map[int]*stat, minRatings int) (int, error) {
	// Several MovieLens ids can map to one TMDB id; merge them.
	byTMDB := map[int]*stat{}
	for _, s := range stats {
		m := byTMDB[s.tmdbID]
		if m == nil {
			m = &stat{tmdbID: s.tmdbID}
			byTMDB[s.tmdbID] = m
		}
		m.sum, m.sumSq, m.n = m.sum+s.sum, m.sumSq+s.sumSq, m.n+s.n
	}
	var rows [][]any
	for id, s := range byTMDB {
		if s.n < float64(minRatings) {
			continue
		}
		mean := s.sum / s.n
		std := math.Sqrt(math.Max(0, s.sumSq/s.n-mean*mean))
		rows = append(rows, []any{id, "movielens", float32(mean), float32(std), int(s.n)})
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `DELETE FROM film_stats WHERE source = 'movielens'`); err != nil {
		return 0, err
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"film_stats"},
		[]string{"tmdb_id", "source", "rating_mean", "rating_std", "rating_count"}, pgx.CopyFromRows(rows)); err != nil {
		return 0, err
	}
	return len(rows), tx.Commit(ctx)
}

/* ── Catalogue ──────────────────────────────────────────────────────────── */

// pendingFilms returns TMDB ids for the top-N films by rating count that are
// missing or stale, plus every existing film below the current text version.
func pendingFilms(ctx context.Context, pool *pgxpool.Pool, top int) ([]int, error) {
	rows, err := pool.Query(ctx, `
		WITH top AS (
			SELECT tmdb_id FROM film_stats ORDER BY rating_count DESC LIMIT $1
		)
		SELECT t.tmdb_id FROM top t
		LEFT JOIN films f ON f.tmdb_id = t.tmdb_id
		WHERE f.id IS NULL OR f.embedding IS NULL OR f.embedding_version < $2
		UNION
		SELECT tmdb_id FROM films WHERE embedding_version < $2 OR embedding IS NULL`,
		top, embedding.TextVersion)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int])
	sort.Ints(ids)
	return ids, err
}

func buildFilm(ctx context.Context, pool *pgxpool.Pool, tmdb *enrichment.TMDBClient, embedder *embedding.Client, tmdbID int) error {
	d, err := tmdb.GetFilmDetails(ctx, tmdbID)
	if err != nil {
		return err
	}
	if d.Title == "" || d.Year == 0 {
		return fmt.Errorf("incomplete TMDB record")
	}
	text := embedding.Text(embedding.Film{
		Title: d.Title, Year: d.Year, Genres: d.Genres, Directors: d.Directors,
		Keywords: d.Keywords, Synopsis: d.Synopsis, Language: d.Language, Country: d.Country,
	}, embedding.TextVersion)
	vec, err := embedder.Embed(ctx, text)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO films (tmdb_id, title, year, synopsis, genres, directors, themes,
		                   language, country, runtime, popularity, poster_path,
		                   embedding, embedding_computed, embedding_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''), $10, $11, NULLIF($12, ''),
		        $13::vector, true, $14)
		ON CONFLICT (tmdb_id) DO UPDATE SET
			synopsis = EXCLUDED.synopsis, genres = EXCLUDED.genres, directors = EXCLUDED.directors,
			themes = EXCLUDED.themes, language = EXCLUDED.language, country = EXCLUDED.country,
			runtime = EXCLUDED.runtime, popularity = EXCLUDED.popularity,
			poster_path = COALESCE(EXCLUDED.poster_path, films.poster_path),
			embedding = EXCLUDED.embedding, embedding_computed = true,
			embedding_version = EXCLUDED.embedding_version`,
		d.TMDBId, d.Title, d.Year, d.Synopsis, d.Genres, d.Directors, d.Keywords,
		d.Language, d.Country, d.Runtime, d.Popularity, d.PosterPath,
		embedding.FormatVector(vec), embedding.TextVersion)
	return err
}

// markQuizPool picks the quiz cards: a few of the most-rated films as
// warm-ups (people like recognising things), then the films that best split
// opinion among those widely seen: rating spread × log(rating count). A film
// everyone loves tells us little about you; one people disagree on tells a lot.
func markQuizPool(ctx context.Context, pool *pgxpool.Pool, size int) (int64, error) {
	warmups := size / 7
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `UPDATE films SET in_quiz_pool = false WHERE in_quiz_pool`); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, `
		WITH ranked AS (
			SELECT f.id, s.rating_std * ln(s.rating_count) AS info,
			       row_number() OVER (ORDER BY s.rating_count DESC) AS pop_rank
			FROM films f JOIN film_stats s ON s.tmdb_id = f.tmdb_id
			WHERE f.embedding_version >= $3 AND f.embedding IS NOT NULL
			  AND f.poster_path IS NOT NULL AND f.synopsis <> ''
		),
		pick AS (
			SELECT id FROM ranked WHERE pop_rank <= $1
			UNION
			(SELECT id FROM ranked WHERE pop_rank > $1 AND pop_rank <= 1500 ORDER BY info DESC LIMIT $2)
		)
		UPDATE films SET in_quiz_pool = true WHERE id IN (SELECT id FROM pick)`,
		warmups, size-warmups, embedding.TextVersion)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), tx.Commit(ctx)
}

// storeCatalogueMean writes the popularity-weighted mean film embedding that
// taste.Production subtracts before weighting. Weighting by how often a film
// is rated makes it "the average film people watch", not the average of an
// arbitrary catalogue list.
func storeCatalogueMean(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		SELECT f.embedding::text, s.rating_count FROM films f
		JOIN film_stats s ON s.tmdb_id = f.tmdb_id
		WHERE f.embedding IS NOT NULL AND f.embedding_version = $1`, embedding.TextVersion)
	if err != nil {
		return err
	}
	defer rows.Close()
	var acc []float64
	var total float64
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return err
		}
		v, err := embedding.ParseVector(s)
		if err != nil {
			return err
		}
		if acc == nil {
			acc = make([]float64, len(v))
		}
		for i, x := range v {
			acc[i] += float64(n) * float64(x)
		}
		total += float64(n)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if total == 0 {
		return fmt.Errorf("no catalogue films with stats to average")
	}
	mean := make([]float32, len(acc))
	for i := range acc {
		mean[i] = float32(acc[i] / total)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO taste_reference (name, vector) VALUES ('catalogue_mean', $1::vector)
		ON CONFLICT (name) DO UPDATE SET vector = EXCLUDED.vector, updated_at = now()`,
		embedding.FormatVector(mean))
	if err == nil {
		log.Printf("[catalogue] stored catalogue_mean")
	}
	return err
}

func requeueVectors(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `SELECT DISTINCT user_id FROM user_ratings`)
	if err != nil {
		return err
	}
	users, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: envOr("REDIS_URL", "localhost:6379")})
	defer client.Close()
	for _, u := range users {
		t, err := jobs.NewComputeVectorTask(u)
		if err != nil {
			return err
		}
		if _, err := client.Enqueue(t); err != nil {
			return err
		}
	}
	log.Printf("[catalogue] queued taste-vector recompute for %d users", len(users))
	return nil
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("%s is not set", k)
	}
	return v
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
