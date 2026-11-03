package handlers

import (
	"log"

	"github.com/framed-app/api/internal/jobs"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type filmSummary struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

type matchOtherUser struct {
	ID               string `json:"id"`
	DisplayName      string `json:"display_name"`
	LetterboxdHandle string `json:"letterboxd_handle"`
	Age              int    `json:"age"`
	LocationCity     string `json:"location_city"`
	LocationCountry  string `json:"location_country"`
}

type matchListItem struct {
	ID            string         `json:"id"`
	Score         float64        `json:"score"`
	WhyText       string         `json:"why_text,omitempty"`
	Status        string         `json:"status"`
	OtherUser     matchOtherUser `json:"other_user"`
	FirstDateFilm *filmSummary   `json:"first_date_film,omitempty"`
}

type ratedFilm struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Year        int     `json:"year"`
	YourRating  float64 `json:"your_rating"`
	TheirRating float64 `json:"their_rating"`
}

// Match (POST /match) — compute + persist matches for a user.
func (h *Handlers) Match(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	log.Printf("[match] finding matches for user=%s", userID)

	matches, err := h.matcher.FindMatches(c.Context(), userID, 20)
	if err != nil {
		log.Printf("[match] error for user=%s err=%v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to compute matches",
		})
	}

	log.Printf("[match] found %d matches for user=%s", len(matches), userID)

	if len(matches) > 0 {
		candidateIDs := make([]string, len(matches))
		for i, m := range matches {
			if m.UserA == userID {
				candidateIDs[i] = m.UserB
			} else {
				candidateIDs[i] = m.UserA
			}
		}
		if task, err := jobs.NewGenerateBlurbsTask(userID, candidateIDs); err == nil {
			if _, err = h.asynqClient.Enqueue(task); err != nil {
				log.Printf("[match] failed to enqueue blurbs for user=%s err=%v", userID, err)
			}
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"matches": matches,
		"count":   len(matches),
	})
}

// GetMatches (GET /matches) — return stored matches for the authenticated user.
func (h *Handlers) GetMatches(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	rows, err := h.pool.Query(c.Context(), `
		SELECT
			m.id::text,
			m.score,
			COALESCE(m.why_text, ''),
			m.status,
			CASE WHEN m.user_a = $1::uuid THEN m.user_b::text ELSE m.user_a::text END AS other_id,
			COALESCE(u.age, 0),
			u.location_city,
			u.location_country,
			COALESCE(sj.letterboxd_handle, ''),
			COALESCE(NULLIF(u.display_name, ''), NULLIF(sj.letterboxd_handle, ''), 'Film lover'),
			COALESCE(f.id::text, ''),
			COALESCE(f.title, ''),
			COALESCE(f.year, 0)
		FROM matches m
		JOIN users u ON u.id = CASE WHEN m.user_a = $1::uuid THEN m.user_b ELSE m.user_a END
		LEFT JOIN LATERAL (
			SELECT letterboxd_handle FROM scrape_jobs
			WHERE user_id = u.id ORDER BY created_at DESC LIMIT 1
		) sj ON true
		LEFT JOIN films f ON f.id = m.first_date_film_id
		WHERE m.user_a = $1::uuid OR m.user_b = $1::uuid
		ORDER BY m.score DESC
	`, userID)
	if err != nil {
		log.Printf("[matches] query error for user=%s err=%v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch matches"})
	}
	defer rows.Close()

	var items []matchListItem
	for rows.Next() {
		var item matchListItem
		var filmID, filmTitle string
		var filmYear int
		if err := rows.Scan(
			&item.ID, &item.Score, &item.WhyText, &item.Status,
			&item.OtherUser.ID, &item.OtherUser.Age,
			&item.OtherUser.LocationCity, &item.OtherUser.LocationCountry,
			&item.OtherUser.LetterboxdHandle,
			&item.OtherUser.DisplayName,
			&filmID, &filmTitle, &filmYear,
		); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "scan error"})
		}
		if filmID != "" {
			item.FirstDateFilm = &filmSummary{ID: filmID, Title: filmTitle, Year: filmYear}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "rows error"})
	}

	if items == nil {
		items = []matchListItem{}
	}
	return c.JSON(fiber.Map{"matches": items, "count": len(items)})
}

// GetMatch (GET /match/:matchId) — return full match detail with shared and disagreement films.
func (h *Handlers) GetMatch(c *fiber.Ctx) error {
	matchID := c.Params("matchId")
	requestingUserID, _ := c.Locals("userID").(string)
	if matchID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "matchId param is required"})
	}

	// ── Fetch match header ───────────────────────────────────────────────────
	var item matchListItem
	var userA, userB string
	var ageA, ageB int
	var cityA, countryA, handleA, nameA string
	var cityB, countryB, handleB, nameB string
	var filmID, filmTitle string
	var filmYear int

	err := h.pool.QueryRow(c.Context(), `
		SELECT
			m.id::text,
			m.score,
			COALESCE(m.why_text, ''),
			m.status,
			m.user_a::text,
			m.user_b::text,
			COALESCE(u_a.age, 0), u_a.location_city, u_a.location_country,
			COALESCE(sj_a.letterboxd_handle, ''),
			COALESCE(NULLIF(u_a.display_name, ''), NULLIF(sj_a.letterboxd_handle, ''), 'Film lover'),
			COALESCE(u_b.age, 0), u_b.location_city, u_b.location_country,
			COALESCE(sj_b.letterboxd_handle, ''),
			COALESCE(NULLIF(u_b.display_name, ''), NULLIF(sj_b.letterboxd_handle, ''), 'Film lover'),
			COALESCE(f.id::text, ''),
			COALESCE(f.title, ''),
			COALESCE(f.year, 0)
		FROM matches m
		JOIN users u_a ON u_a.id = m.user_a
		JOIN users u_b ON u_b.id = m.user_b
		LEFT JOIN LATERAL (
			SELECT letterboxd_handle FROM scrape_jobs WHERE user_id = m.user_a ORDER BY created_at DESC LIMIT 1
		) sj_a ON true
		LEFT JOIN LATERAL (
			SELECT letterboxd_handle FROM scrape_jobs WHERE user_id = m.user_b ORDER BY created_at DESC LIMIT 1
		) sj_b ON true
		LEFT JOIN films f ON f.id = m.first_date_film_id
		WHERE m.id = $1::uuid AND (m.user_a = $2::uuid OR m.user_b = $2::uuid)
	`, matchID, requestingUserID).Scan(
		&item.ID, &item.Score, &item.WhyText, &item.Status,
		&userA, &userB,
		&ageA, &cityA, &countryA, &handleA, &nameA,
		&ageB, &cityB, &countryB, &handleB, &nameB,
		&filmID, &filmTitle, &filmYear,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "match not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch match"})
	}

	// Assign "me" vs "other" based on the requesting user.
	var myID, theirID string
	var theirAge int
	var theirCity, theirCountry, theirHandle, theirName string
	if requestingUserID == userA {
		myID, theirID = userA, userB
		theirAge, theirCity, theirCountry, theirHandle, theirName = ageB, cityB, countryB, handleB, nameB
	} else {
		myID, theirID = userB, userA
		theirAge, theirCity, theirCountry, theirHandle, theirName = ageA, cityA, countryA, handleA, nameA
	}
	item.OtherUser = matchOtherUser{
		ID:               theirID,
		DisplayName:      theirName,
		LetterboxdHandle: theirHandle,
		Age:              theirAge,
		LocationCity:     theirCity,
		LocationCountry:  theirCountry,
	}
	if filmID != "" {
		item.FirstDateFilm = &filmSummary{ID: filmID, Title: filmTitle, Year: filmYear}
	}

	// ── Shared films (both rated ≥ 3.5) ─────────────────────────────────────
	sharedFilms, err := h.fetchRatedFilms(c, myID, theirID, `
		SELECT f.id::text, f.title, f.year, ur_me.rating, ur_them.rating
		FROM user_ratings ur_me
		JOIN user_ratings ur_them ON ur_them.film_id = ur_me.film_id AND ur_them.user_id = $2
		JOIN films f ON f.id = ur_me.film_id
		WHERE ur_me.user_id = $1 AND ur_me.rating >= 3.5 AND ur_them.rating >= 3.5
		ORDER BY (ur_me.rating + ur_them.rating) DESC
		LIMIT 10
	`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch shared films"})
	}

	// ── Disagreement films (rating gap ≥ 2.0) ───────────────────────────────
	disagreements, err := h.fetchRatedFilms(c, myID, theirID, `
		SELECT f.id::text, f.title, f.year, ur_me.rating, ur_them.rating
		FROM user_ratings ur_me
		JOIN user_ratings ur_them ON ur_them.film_id = ur_me.film_id AND ur_them.user_id = $2
		JOIN films f ON f.id = ur_me.film_id
		WHERE ur_me.user_id = $1 AND ABS(ur_me.rating - ur_them.rating) >= 2.0
		ORDER BY ABS(ur_me.rating - ur_them.rating) DESC
		LIMIT 5
	`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch disagreements"})
	}

	return c.JSON(fiber.Map{
		"id":                 item.ID,
		"score":              item.Score,
		"why_text":           item.WhyText,
		"status":             item.Status,
		"other_user":         item.OtherUser,
		"first_date_film":    item.FirstDateFilm,
		"shared_films":       sharedFilms,
		"disagreement_films": disagreements,
	})
}

func (h *Handlers) fetchRatedFilms(c *fiber.Ctx, myID, theirID, query string) ([]ratedFilm, error) {
	rows, err := h.pool.Query(c.Context(), query, myID, theirID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var films []ratedFilm
	for rows.Next() {
		var f ratedFilm
		if err := rows.Scan(&f.ID, &f.Title, &f.Year, &f.YourRating, &f.TheirRating); err != nil {
			return nil, err
		}
		films = append(films, f)
	}
	if films == nil {
		films = []ratedFilm{}
	}
	return films, rows.Err()
}
