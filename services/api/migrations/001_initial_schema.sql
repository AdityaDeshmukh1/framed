-- Migration: 001_initial_schema
-- Run this once against a fresh database.
-- Requires: pgvector extension (installed via docker image pgvector/pgvector:pg16)

-- ── Extensions ────────────────────────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";   -- uuid_generate_v4()
CREATE EXTENSION IF NOT EXISTS vector;         -- pgvector

-- ── Users ─────────────────────────────────────────────────────────────────────
CREATE TABLE users (
  id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  email               TEXT UNIQUE NOT NULL,
  age                 INTEGER NOT NULL CHECK (age >= 18),
  -- location stored as fuzzed lat/lng — never exact GPS
  location_lat        FLOAT,
  location_lng        FLOAT,
  location_city       TEXT NOT NULL DEFAULT '',
  location_country    TEXT NOT NULL DEFAULT '',
  distance_mode       TEXT NOT NULL DEFAULT 'local'
                        CHECK (distance_mode IN ('local','regional','national','global')),
  distance_km         INTEGER,                          -- NULL means no limit
  seeking             TEXT[] NOT NULL DEFAULT '{}',
  intentions          TEXT[] NOT NULL DEFAULT '{}',
  now_showing         TEXT CHECK (char_length(now_showing) <= 150),
  director_note       TEXT CHECK (char_length(director_note) <= 150),
  visibility          TEXT NOT NULL DEFAULT 'discoverable'
                        CHECK (visibility IN ('discoverable','matches_only','hidden')),
  onboarding_complete BOOLEAN NOT NULL DEFAULT FALSE,
  last_active         TIMESTAMPTZ,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_prompts (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  prompt_key    TEXT NOT NULL,
  response      TEXT NOT NULL CHECK (char_length(response) <= 200),
  display_order INTEGER NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(user_id, prompt_key)
);

-- ── Films ─────────────────────────────────────────────────────────────────────
-- Shared catalogue. Enriched once from TMDB, reused across all users.
CREATE TABLE films (
  id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  tmdb_id               INTEGER UNIQUE NOT NULL,
  letterboxd_slug       TEXT UNIQUE NOT NULL,
  title                 TEXT NOT NULL,
  year                  INTEGER NOT NULL,
  directors             TEXT[] NOT NULL DEFAULT '{}',
  genres                TEXT[] NOT NULL DEFAULT '{}',
  themes                TEXT[] NOT NULL DEFAULT '{}',
  tone                  TEXT[] NOT NULL DEFAULT '{}',
  runtime               INTEGER,
  language              TEXT,
  country               TEXT,
  poster_path           TEXT,
  synopsis              TEXT NOT NULL DEFAULT '',
  avg_letterboxd_rating FLOAT NOT NULL DEFAULT 0,
  popularity            FLOAT NOT NULL DEFAULT 0,
  -- 1536-dimensional embedding from text-embedding-3-small
  -- represents the semantic content of synopsis + themes + tone
  embedding             vector(1536),
  embedding_computed    BOOLEAN NOT NULL DEFAULT FALSE,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- HNSW index on film embeddings for fast nearest-neighbour search
-- Built after initial data load, not at table creation
-- CREATE INDEX ON films USING hnsw (embedding vector_cosine_ops);

-- ── User Ratings ──────────────────────────────────────────────────────────────
CREATE TABLE user_ratings (
  id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  film_id      UUID NOT NULL REFERENCES films(id) ON DELETE CASCADE,
  rating       NUMERIC(2,1) NOT NULL CHECK (rating >= 0.5 AND rating <= 5.0),
  watched_date DATE,
  rewatch      BOOLEAN NOT NULL DEFAULT FALSE,
  liked        BOOLEAN NOT NULL DEFAULT FALSE,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(user_id, film_id)
);

CREATE INDEX idx_user_ratings_user_id ON user_ratings(user_id);
CREATE INDEX idx_user_ratings_film_id ON user_ratings(film_id);

-- ── Taste Vectors ─────────────────────────────────────────────────────────────
-- One row per user. Recomputed when ratings change beyond threshold.
CREATE TABLE taste_vectors (
  id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE UNIQUE,
  -- 256-dimensional taste vector, projected down from film embeddings
  vector           vector(256) NOT NULL,
  top_genres       TEXT[] NOT NULL DEFAULT '{}',
  top_directors    TEXT[] NOT NULL DEFAULT '{}',
  top_decades      INTEGER[] NOT NULL DEFAULT '{}',
  avg_rating       FLOAT NOT NULL DEFAULT 0,
  rating_variance  FLOAT NOT NULL DEFAULT 0,
  obscurity_score  FLOAT NOT NULL DEFAULT 0 CHECK (obscurity_score >= 0 AND obscurity_score <= 1),
  total_films      INTEGER NOT NULL DEFAULT 0,
  computed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- HNSW index — this is what makes matching fast at scale
-- <=> is cosine distance operator in pgvector
CREATE INDEX idx_taste_vectors_hnsw ON taste_vectors
  USING hnsw (vector vector_cosine_ops)
  WITH (m = 16, ef_construction = 64);

-- ── Soul Portraits ────────────────────────────────────────────────────────────
CREATE TABLE soul_portraits (
  id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id                 UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE UNIQUE,
  portrait_text           TEXT NOT NULL,
  summary_line            TEXT NOT NULL,
  unseen_recommendations  TEXT[] NOT NULL DEFAULT '{}',
  user_accuracy_rating    TEXT CHECK (user_accuracy_rating IN
                            ('nailed_it','mostly_right','bit_off','who_is_this')),
  user_accuracy_note      TEXT CHECK (char_length(user_accuracy_note) <= 100),
  -- snapshot of the taste vector at generation time (256d)
  -- used to detect drift and decide if regeneration is warranted
  vector_snapshot         vector(256),
  computed_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Matches ───────────────────────────────────────────────────────────────────
CREATE TABLE matches (
  id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_a              UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  user_b              UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  score               FLOAT NOT NULL,
  why_text            TEXT,
  first_date_film_id  UUID REFERENCES films(id),
  status              TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','mutual','passed')),
  user_a_action       TEXT CHECK (user_a_action IN ('liked','passed')),
  user_b_action       TEXT CHECK (user_b_action IN ('liked','passed')),
  matched_at          TIMESTAMPTZ,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- enforce canonical ordering: user_a < user_b (by UUID string)
  -- prevents duplicate rows for the same pair
  UNIQUE(user_a, user_b),
  CHECK (user_a <> user_b)
);

CREATE INDEX idx_matches_user_a ON matches(user_a);
CREATE INDEX idx_matches_user_b ON matches(user_b);
CREATE INDEX idx_matches_status ON matches(status);

-- ── Conversations + Messages ──────────────────────────────────────────────────
CREATE TABLE conversations (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  match_id   UUID NOT NULL REFERENCES matches(id) ON DELETE CASCADE UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE messages (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  sender_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  content         TEXT NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);

-- ── Scrape Jobs ───────────────────────────────────────────────────────────────
-- Tracks the status of profile scrape jobs so we can show progress to users
-- and retry failures without re-queuing duplicates.
CREATE TABLE scrape_jobs (
  id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  letterboxd_handle TEXT NOT NULL,
  status            TEXT NOT NULL DEFAULT 'queued'
                      CHECK (status IN ('queued','scraping','enriching','vectorising','generating','complete','failed')),
  films_found       INTEGER NOT NULL DEFAULT 0,
  films_processed   INTEGER NOT NULL DEFAULT 0,
  error_message     TEXT,
  started_at        TIMESTAMPTZ,
  completed_at      TIMESTAMPTZ,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scrape_jobs_user_id ON scrape_jobs(user_id);