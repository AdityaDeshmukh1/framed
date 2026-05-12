package models

import (
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// User
// ─────────────────────────────────────────────────────────────────────────────

type User struct {
	ID                 uuid.UUID  `db:"id"                  json:"id"`
	Email              string     `db:"email"               json:"-"` // never exposed in API responses
	Age                int        `db:"age"                 json:"age"`
	LocationCity       string     `db:"location_city"       json:"locationCity"`
	LocationCountry    string     `db:"location_country"    json:"locationCountry"`
	DistanceMode       string     `db:"distance_mode"       json:"distanceMode"`
	DistanceKM         *int       `db:"distance_km"         json:"distanceKm"`
	Seeking            []string   `db:"seeking"             json:"seeking"`
	Intentions         []string   `db:"intentions"          json:"intentions"`
	NowShowing         *string    `db:"now_showing"         json:"nowShowing"`
	DirectorNote       *string    `db:"director_note"       json:"directorNote"`
	Visibility         string     `db:"visibility"          json:"visibility"`
	OnboardingComplete bool       `db:"onboarding_complete" json:"onboardingComplete"`
	LastActive         *time.Time `db:"last_active"         json:"lastActive"`
	CreatedAt          time.Time  `db:"created_at"          json:"createdAt"`
}

type UserPrompt struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	UserID       uuid.UUID `db:"user_id"       json:"userId"`
	PromptKey    string    `db:"prompt_key"    json:"promptKey"`
	Response     string    `db:"response"      json:"response"`
	DisplayOrder int       `db:"display_order" json:"displayOrder"`
	CreatedAt    time.Time `db:"created_at"    json:"createdAt"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Film
// The shared catalogue. Enriched once from TMDB, reused across all users.
// ─────────────────────────────────────────────────────────────────────────────

type Film struct {
	ID                  uuid.UUID `db:"id"                    json:"id"`
	TMDBId              int       `db:"tmdb_id"               json:"tmdbId"`
	LetterboxdSlug      string    `db:"letterboxd_slug"       json:"letterboxdSlug"`
	Title               string    `db:"title"                 json:"title"`
	Year                int       `db:"year"                  json:"year"`
	Directors           []string  `db:"directors"             json:"directors"`
	Genres              []string  `db:"genres"                json:"genres"`
	Themes              []string  `db:"themes"                json:"themes"`
	Tone                []string  `db:"tone"                  json:"tone"`
	Runtime             int       `db:"runtime"               json:"runtime"`
	Language            string    `db:"language"              json:"language"`
	Country             string    `db:"country"               json:"country"`
	PosterPath          *string   `db:"poster_path"           json:"posterPath"`
	Synopsis            string    `db:"synopsis"              json:"synopsis"`
	AvgLetterboxdRating float64   `db:"avg_letterboxd_rating" json:"avgLetterboxdRating"`
	Popularity          float64   `db:"popularity"            json:"popularity"`
	// Embedding stored as []float32 in pgvector — not exposed in JSON
	// The embedding is computed once and used only internally for similarity
	EmbeddingComputed bool      `db:"embedding_computed" json:"embeddingComputed"`
	CreatedAt         time.Time `db:"created_at"         json:"createdAt"`
}

// ─────────────────────────────────────────────────────────────────────────────
// UserRating
// Raw material. Everything derives from here.
// ─────────────────────────────────────────────────────────────────────────────

type UserRating struct {
	ID          uuid.UUID  `db:"id"           json:"id"`
	UserID      uuid.UUID  `db:"user_id"      json:"userId"`
	FilmID      uuid.UUID  `db:"film_id"      json:"filmId"`
	Rating      float32    `db:"rating"       json:"rating"` // 0.5 to 5.0
	WatchedDate *time.Time `db:"watched_date" json:"watchedDate"`
	Rewatch     bool       `db:"rewatch"      json:"rewatch"`
	Liked       bool       `db:"liked"        json:"liked"` // letterboxd heart
	CreatedAt   time.Time  `db:"created_at"   json:"createdAt"`
}

// ─────────────────────────────────────────────────────────────────────────────
// TasteVector
// Derived from UserRatings. The pgvector core.
// Stored separately so it can be recomputed without touching the user record.
// ─────────────────────────────────────────────────────────────────────────────

type TasteVector struct {
	ID     uuid.UUID `db:"id"               json:"id"`
	UserID uuid.UUID `db:"user_id"          json:"userId"`
	// Vector itself stored in postgres as vector(256)
	// We pass it as []float32 in Go
	TopGenres      []string  `db:"top_genres"       json:"topGenres"`
	TopDirectors   []string  `db:"top_directors"    json:"topDirectors"`
	TopDecades     []int     `db:"top_decades"      json:"topDecades"`
	AvgRating      float64   `db:"avg_rating"       json:"avgRating"`
	RatingVariance float64   `db:"rating_variance"  json:"ratingVariance"`
	ObscurityScore float64   `db:"obscurity_score"  json:"obscurityScore"` // 0-1
	TotalFilms     int       `db:"total_films"      json:"totalFilms"`
	ComputedAt     time.Time `db:"computed_at"      json:"computedAt"`
}

// ─────────────────────────────────────────────────────────────────────────────
// SoulPortrait
// AI-generated narrative. Stored, never computed live.
// ─────────────────────────────────────────────────────────────────────────────

type SoulPortrait struct {
	ID                    uuid.UUID `db:"id"                         json:"id"`
	UserID                uuid.UUID `db:"user_id"                    json:"userId"`
	PortraitText          string    `db:"portrait_text"              json:"portraitText"`
	SummaryLine           string    `db:"summary_line"               json:"summaryLine"`
	UnseenRecommendations []string  `db:"unseen_recommendations"     json:"unseenRecommendations"`
	UserAccuracyRating    *string   `db:"user_accuracy_rating"       json:"userAccuracyRating"`
	UserAccuracyNote      *string   `db:"user_accuracy_note"         json:"userAccuracyNote"`
	ComputedAt            time.Time `db:"computed_at"                json:"computedAt"`
	// Snapshot of the vector at generation time
	// Used to detect drift and decide if regeneration is needed
	// Not exposed in API — internal bookkeeping only
}

// ─────────────────────────────────────────────────────────────────────────────
// Match
// ─────────────────────────────────────────────────────────────────────────────

type Match struct {
	ID              uuid.UUID   `db:"id"                 json:"id"`
	UserA           uuid.UUID   `db:"user_a"             json:"userA"`
	UserB           uuid.UUID   `db:"user_b"             json:"userB"`
	Score           float64     `db:"score"              json:"score"`
	WhyText         *string     `db:"why_text"           json:"whyText"`
	FirstDateFilmID *uuid.UUID  `db:"first_date_film_id" json:"firstDateFilmId"`
	Status          MatchStatus `db:"status"            json:"status"`
	UserAAction     *string     `db:"user_a_action"      json:"userAAction"`
	UserBAction     *string     `db:"user_b_action"      json:"userBAction"`
	MatchedAt       *time.Time  `db:"matched_at"         json:"matchedAt"`
	CreatedAt       time.Time   `db:"created_at"         json:"createdAt"`
}

type MatchStatus string

const (
	MatchStatusPending MatchStatus = "pending"
	MatchStatusMutual  MatchStatus = "mutual"
	MatchStatusPassed  MatchStatus = "passed"
)

type MatchAction string

const (
	MatchActionLiked  MatchAction = "liked"
	MatchActionPassed MatchAction = "passed"
)

// ─────────────────────────────────────────────────────────────────────────────
// Conversation + Message
// ─────────────────────────────────────────────────────────────────────────────

type Conversation struct {
	ID        uuid.UUID `db:"id"         json:"id"`
	MatchID   uuid.UUID `db:"match_id"   json:"matchId"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

type Message struct {
	ID             uuid.UUID `db:"id"              json:"id"`
	ConversationID uuid.UUID `db:"conversation_id" json:"conversationId"`
	SenderID       uuid.UUID `db:"sender_id"       json:"senderId"`
	Content        string    `db:"content"         json:"content"`
	CreatedAt      time.Time `db:"created_at"      json:"createdAt"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Job
// Represents a background task in the processing pipeline.
// Stored in Redis via Asynq — this struct is for documentation purposes.
// ─────────────────────────────────────────────────────────────────────────────

type JobType string

const (
	JobTypeScrapeProfile  JobType = "scrape:profile"
	JobTypeEnrichFilm     JobType = "enrich:film"
	JobTypeComputeVector  JobType = "vector:compute"
	JobTypeGenerateSoul   JobType = "soul:generate"
	JobTypeGenerateBlurbs JobType = "blurbs:generate"
)

// ScrapeProfilePayload is the data passed to the scrape worker.
type ScrapeProfilePayload struct {
	UserID           string `json:"userId"`
	LetterboxdHandle string `json:"letterboxdHandle"`
}

// EnrichFilmPayload is the data passed to the TMDB enrichment worker.
type EnrichFilmPayload struct {
	TMDBMovieID    int    `json:"tmdbMovieId"`
	FilmID         string `json:"filmId"`
	LetterboxdSlug string `json:"letterboxdSlug"`
}

// ComputeVectorPayload triggers taste vector recomputation for a user.
type ComputeVectorPayload struct {
	UserID string `json:"userId"`
}

// GenerateSoulPayload triggers Soul portrait generation for a user.
type GenerateSoulPayload struct {
	UserID string `json:"userId"`
}

// GenerateBlurbsPayload triggers pairwise "why you'd click" generation.
type GenerateBlurbsPayload struct {
	UserID string `json:"userId"`
	// Top N match candidates to generate blurbs for
	CandidateIDs []string `json:"candidateIds"`
}
