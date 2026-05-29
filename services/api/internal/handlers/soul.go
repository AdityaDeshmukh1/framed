package handlers

import (
	"log"
	"strings"
	"time"

	"github.com/framed-app/api/internal/llm"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// Soul generates a Cinematic Soul portrait for the authenticated user.
func (h *Handlers) Soul(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	log.Printf("[soul] generating portrait for user=%s", userID)

	// Fetch top genres and directors from taste_vectors.
	var topGenres, topDirectors []string
	if err := h.pool.QueryRow(c.Context(),
		`SELECT top_genres, top_directors FROM taste_vectors WHERE user_id = $1`,
		userID).Scan(&topGenres, &topDirectors); err != nil {
		log.Printf("[soul] taste vector not found for user=%s err=%v", userID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "taste vector not found — run matching first"})
	}

	// Fetch top 5 highest-rated films.
	highRatedFilms, err := h.fetchFilmTitles(c, userID, `
		SELECT f.title FROM user_ratings ur
		JOIN films f ON f.id = ur.film_id
		WHERE ur.user_id = $1 AND ur.rating IS NOT NULL
		ORDER BY ur.rating DESC LIMIT 5`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch ratings"})
	}

	// Fetch bottom 5 lowest-rated films.
	lowRatedFilms, err := h.fetchFilmTitles(c, userID, `
		SELECT f.title FROM user_ratings ur
		JOIN films f ON f.id = ur.film_id
		WHERE ur.user_id = $1 AND ur.rating IS NOT NULL
		ORDER BY ur.rating ASC LIMIT 5`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch ratings"})
	}

	// Fetch rewatched films.
	rewatchedFilms, err := h.fetchFilmTitles(c, userID, `
		SELECT f.title FROM user_ratings ur
		JOIN films f ON f.id = ur.film_id
		WHERE ur.user_id = $1 AND ur.rewatch = true LIMIT 10`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch rewatches"})
	}

	portrait, err := llm.GenerateSoul(c.Context(), h.anthropicKey, topGenres, topDirectors, highRatedFilms, lowRatedFilms, rewatchedFilms)
	if err != nil {
		log.Printf("[soul] generation failed for user=%s err=%v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "portrait generation failed"})
	}

	// Use the first sentence as the summary line.
	summaryLine := firstSentence(portrait)

	if _, err = h.pool.Exec(c.Context(),
		`INSERT INTO soul_portraits (user_id, portrait_text, summary_line)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE
		   SET portrait_text = EXCLUDED.portrait_text,
		       summary_line  = EXCLUDED.summary_line,
		       computed_at   = now()`,
		userID, portrait, summaryLine); err != nil {
		log.Printf("[soul] failed to store portrait for user=%s err=%v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to store portrait"})
	}

	log.Printf("[soul] completed portrait for user=%s", userID)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"portrait": portrait})
}

// fetchFilmTitles runs a query that returns film titles for a user.
func (h *Handlers) fetchFilmTitles(c *fiber.Ctx, userID, query string) ([]string, error) {
	rows, err := h.pool.Query(c.Context(), query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var titles []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		titles = append(titles, t)
	}
	return titles, rows.Err()
}

// GetSoul fetches an existing soul portrait for the authenticated user.
func (h *Handlers) GetSoul(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	var portrait, summaryLine string
	var computedAt time.Time
	err := h.pool.QueryRow(c.Context(),
		`SELECT portrait_text, summary_line, computed_at FROM soul_portraits WHERE user_id = $1`,
		userID).Scan(&portrait, &summaryLine, &computedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "portrait not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch portrait"})
	}

	return c.JSON(fiber.Map{
		"portrait":     portrait,
		"summary_line": summaryLine,
		"computed_at":  computedAt,
	})
}

// firstSentence returns the text up to and including the first sentence-ending punctuation.
func firstSentence(s string) string {
	for i, r := range s {
		if r == '.' || r == '!' || r == '?' {
			return strings.TrimSpace(s[:i+1])
		}
	}
	// Fallback: truncate to 150 chars (soul_portraits.summary_line has no length constraint beyond check).
	if len(s) > 150 {
		return s[:150]
	}
	return s
}
