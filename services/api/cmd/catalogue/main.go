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
	"strings"
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
	perDecade := flag.Int("per-decade", 250, "also pre-load the N most-rated films of each decade, so recent and older films aren't crowded out by 1990s-2000s MovieLens favourites")
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
	if *perDecade > 0 {
		extra, err := decadeFilms(ctx, pool, *mlDir, *perDecade)
		if err != nil {
			log.Fatal(err)
		}
		targets = mergeIDs(targets, extra)
	}
	log.Printf("[catalogue] %d films to enrich+embed (top %d by ratings, top %d per decade, plus anything below text v%d)", len(targets), *top, *perDecade, embedding.TextVersion)

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

// readYears maps TMDB id → release year from MovieLens movies.csv titles
// ("Heat (1995)"). film_stats has no year, and most of these films aren't in
// `films` yet, so this is the only place to get it before building them.
func readYears(dir string) (map[int]int, error) {
	links, err := readLinks(filepath.Join(dir, "links.csv"))
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(dir, "movies.csv"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	if _, err := r.Read(); err != nil {
		return nil, err
	}
	years := map[int]int{}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		movie, err := strconv.Atoi(rec[0])
		if err != nil {
			continue
		}
		title := strings.TrimSpace(rec[1])
		if len(title) < 6 || title[len(title)-1] != ')' || title[len(title)-6] != '(' {
			continue
		}
		year, err := strconv.Atoi(title[len(title)-5 : len(title)-1])
		if tmdb, ok := links[movie]; ok && err == nil {
			years[tmdb] = year
		}
	}
	return years, nil
}

// decadeFilms returns the perDecade most-rated films of every decade that
// aren't built yet. Ranking by count overall gives ~75% 1990s-2000s films,
// because that's when MovieLens users were most active.
func decadeFilms(ctx context.Context, pool *pgxpool.Pool, dir string, perDecade int) ([]int, error) {
	years, err := readYears(dir)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT s.tmdb_id, s.rating_count FROM film_stats s
		LEFT JOIN films f ON f.tmdb_id = s.tmdb_id
		WHERE s.rating_count >= 300
		  AND (f.id IS NULL OR f.embedding IS NULL OR f.embedding_version < $1)
		ORDER BY s.rating_count DESC`, embedding.TextVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	taken := map[int]int{}
	var ids []int
	for rows.Next() {
		var id, n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		y, ok := years[id]
		if !ok {
			continue
		}
		if d := decade(y); taken[d] < perDecade {
			taken[d]++
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// decade buckets everything before 1960 together: there are too few
// well-known films per decade to fill a quota on their own.
func decade(year int) int {
	if year < 1960 {
		return 1950
	}
	return year / 10 * 10
}

func mergeIDs(a, b []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, id := range append(a, b...) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Ints(out)
	return out
}

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

// Share of quiz cards per decade (decade() buckets pre-1960 together). Skews
// a little recent, because that's what new users have seen; the pool used to
// be 75% 1990s-2000s.
var decadeShare = map[int]float64{
	1950: 0.07, 1960: 0.06, 1970: 0.08, 1980: 0.12,
	1990: 0.16, 2000: 0.19, 2010: 0.23, 2020: 0.09,
}

const (
	// No genre may appear on more than this share of cards (a film counts
	// toward each of its genres). Action/Comedy/Adventure were ~37% each.
	maxGenreShare = 0.28
	// At least this share of each decade's cards are non-English.
	minForeignShare = 0.15
	// One card in this many is a well-known warm-up, picked by rating count.
	warmupEvery = 7
)

type candidate struct {
	id      string
	decade  int
	english bool
	genres  []string
	count   int
	info    float64 // rating spread × ln(count): divisive and widely seen
}

// markQuizPool picks the quiz cards. Per decade: a few of the most-rated films
// as warm-ups (people like recognising things), then the films that best split
// opinion among those widely seen, rating spread × log(rating count): a film
// everyone loves tells us little about you; one people disagree on tells a
// lot. Genre caps and a non-English minimum keep the mix varied.
func markQuizPool(ctx context.Context, pool *pgxpool.Pool, size int) (int64, error) {
	rows, err := pool.Query(ctx, `
		SELECT f.id::text, f.year, COALESCE(f.language, ''), f.genres, s.rating_count, s.rating_std
		FROM films f JOIN film_stats s ON s.tmdb_id = f.tmdb_id
		WHERE f.embedding_version >= $1 AND f.embedding IS NOT NULL
		  AND f.poster_path IS NOT NULL AND f.synopsis <> '' AND s.rating_count >= 300`,
		embedding.TextVersion)
	if err != nil {
		return 0, err
	}
	byDecade := map[int][]candidate{}
	for rows.Next() {
		var c candidate
		var year int
		var lang string
		var std float64
		if err := rows.Scan(&c.id, &year, &lang, &c.genres, &c.count, &std); err != nil {
			rows.Close()
			return 0, err
		}
		c.decade, c.english = decade(year), lang == "en"
		c.info = std * math.Log(float64(c.count))
		byDecade[c.decade] = append(byDecade[c.decade], c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	picked := map[string]bool{}
	genreCount := map[string]int{}
	genreCap := int(maxGenreShare * float64(size))
	take := func(c candidate, capGenres bool) bool {
		if picked[c.id] {
			return false
		}
		if capGenres {
			for _, g := range c.genres {
				if genreCount[g] >= genreCap {
					return false
				}
			}
		}
		picked[c.id] = true
		for _, g := range c.genres {
			genreCount[g]++
		}
		return true
	}
	// fill takes up to n films from cs (already sorted), returning how many.
	fill := func(cs []candidate, n int, keep func(candidate) bool, capGenres bool) int {
		got := 0
		for _, c := range cs {
			if got >= n {
				break
			}
			if keep(c) && take(c, capGenres) {
				got++
			}
		}
		return got
	}
	anyFilm := func(candidate) bool { return true }

	decades := make([]int, 0, len(decadeShare))
	for d := range decadeShare {
		decades = append(decades, d)
	}
	sort.Ints(decades)
	for _, d := range decades {
		quota := int(math.Round(decadeShare[d] * float64(size)))
		cs := byDecade[d]
		byCount := append([]candidate(nil), cs...)
		sort.Slice(byCount, func(i, j int) bool { return byCount[i].count > byCount[j].count })
		sort.Slice(cs, func(i, j int) bool { return cs[i].info > cs[j].info })

		got := fill(byCount, quota/warmupEvery, anyFilm, true)
		foreign := int(math.Ceil(minForeignShare * float64(quota)))
		got += fill(cs, foreign, func(c candidate) bool { return !c.english }, true)
		got += fill(cs, quota-got, anyFilm, true)
		log.Printf("[catalogue] quiz pool %ds: %d/%d (%d candidates)", d, got, quota, len(cs))
	}
	// Decades that ran short (or rounding) are topped up from anywhere.
	if remaining := size - len(picked); remaining > 0 {
		var all []candidate
		for _, cs := range byDecade {
			all = append(all, cs...)
		}
		sort.Slice(all, func(i, j int) bool { return all[i].info > all[j].info })
		if n := fill(all, remaining, anyFilm, true); n < remaining {
			fill(all, remaining-n, anyFilm, false)
		}
	}

	ids := make([]string, 0, len(picked))
	for id := range picked {
		ids = append(ids, id)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `UPDATE films SET in_quiz_pool = false WHERE in_quiz_pool`); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, `UPDATE films SET in_quiz_pool = true WHERE id = ANY($1::uuid[])`, ids)
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
