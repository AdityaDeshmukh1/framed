-- Which version of embedding.Text produced films.embedding. Films below the
-- current version are re-embedded by `go run ./cmd/catalogue -reembed`.
ALTER TABLE films ADD COLUMN embedding_version INT NOT NULL DEFAULT 1;

-- Films offered as quiz cards during onboarding.
ALTER TABLE films ADD COLUMN in_quiz_pool BOOLEAN NOT NULL DEFAULT false;

-- How the crowd rates each film. Loaded from MovieLens by cmd/catalogue
-- (research/non-commercial licence); later recomputed from Framed's own
-- ratings. Keyed by TMDB id so it can describe films not yet in `films`.
CREATE TABLE film_stats (
    tmdb_id      INT PRIMARY KEY,
    source       TEXT NOT NULL,
    rating_mean  REAL NOT NULL,
    rating_std   REAL NOT NULL,
    rating_count INT NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- How much a taste vector can be trusted, so matching can show "Strong" /
-- "Good" instead of a precise percentage for thin profiles.
ALTER TABLE taste_vectors ADD COLUMN film_count INT NOT NULL DEFAULT 0;
ALTER TABLE taste_vectors ADD COLUMN confidence REAL NOT NULL DEFAULT 0;
