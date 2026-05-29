-- Clerk integration.
-- clerk_id is the primary identifier coming from JWT tokens.
-- Nullable for now so existing local test users aren't broken;
-- enforce NOT NULL once all environments have Clerk wired up.
--
-- email and age are relaxed because at Clerk signup time we only have
-- the Clerk user ID — the rest of the profile is filled in later.
ALTER TABLE users
  ADD COLUMN clerk_id TEXT UNIQUE,
  ALTER COLUMN email DROP NOT NULL,
  ALTER COLUMN age   DROP NOT NULL,
  ALTER COLUMN age   SET DEFAULT NULL,
  DROP CONSTRAINT IF EXISTS users_age_check;

CREATE INDEX IF NOT EXISTS idx_users_clerk_id ON users(clerk_id);
