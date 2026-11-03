package handlers

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"sync"
	"time"

	"github.com/framed-app/api/internal/embedding"
	"github.com/framed-app/api/pkg/db"
	"github.com/gofiber/fiber/v2"
)

// QuizFilm is one quiz card. The client picks cards itself (apps/web
// lib/quiz/engine.ts), so it gets each film's embedding and crowd stats.
type QuizFilm struct {
	TMDBID      int       `json:"tmdbId"`
	Title       string    `json:"title"`
	Year        int       `json:"year"`
	Directors   []string  `json:"directors"`
	Genres      []string  `json:"genres"`
	Language    string    `json:"language"`
	PosterPath  string    `json:"posterPath"`
	RatingMean  float64   `json:"ratingMean"`
	RatingStd   float64   `json:"ratingStd"`
	RatingCount int       `json:"ratingCount"`
	Embedding   []float64 `json:"embedding"`
}

// quizPoolCache holds the encoded pool: it changes only when cmd/catalogue
// runs, and it's ~1MB of JSON we don't want to rebuild per request.
type quizPoolCache struct {
	mu      sync.Mutex
	body    []byte
	builtAt time.Time
}

const quizPoolTTL = 10 * time.Minute

// QuizPool serves GET /quiz/pool.
func (h *Handlers) QuizPool(c *fiber.Ctx) error {
	body, err := h.quizPool.get(c.Context(), h.pool)
	if err != nil {
		log.Printf("[quiz] pool: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load quiz pool"})
	}
	c.Set("Cache-Control", "private, max-age=600")
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Send(body)
}

func (q *quizPoolCache) get(ctx context.Context, pool *db.Pool) ([]byte, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.body != nil && time.Since(q.builtAt) < quizPoolTTL {
		return q.body, nil
	}
	rows, err := pool.Query(ctx, `
		SELECT f.tmdb_id, f.title, f.year, f.directors, f.genres, COALESCE(f.language, ''),
		       f.poster_path, s.rating_mean, s.rating_std, s.rating_count, f.embedding::text
		FROM films f JOIN film_stats s ON s.tmdb_id = f.tmdb_id
		WHERE f.in_quiz_pool AND f.embedding IS NOT NULL
		ORDER BY s.rating_count DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	films := []QuizFilm{}
	for rows.Next() {
		var f QuizFilm
		var emb string
		if err := rows.Scan(&f.TMDBID, &f.Title, &f.Year, &f.Directors, &f.Genres, &f.Language,
			&f.PosterPath, &f.RatingMean, &f.RatingStd, &f.RatingCount, &emb); err != nil {
			return nil, err
		}
		v, err := embedding.ParseVector(emb)
		if err != nil {
			return nil, err
		}
		f.Embedding = make([]float64, len(v))
		for i, x := range v {
			f.Embedding[i] = math.Round(float64(x)*1e4) / 1e4 // 4 dp: halves the payload, no loss that matters
		}
		f.RatingMean = math.Round(f.RatingMean*100) / 100
		f.RatingStd = math.Round(f.RatingStd*100) / 100
		films = append(films, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(films)
	if err != nil {
		return nil, err
	}
	q.body, q.builtAt = body, time.Now()
	return body, nil
}
