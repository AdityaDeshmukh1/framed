-- Onboarding imports can come from several sources (see internal/importer).
-- Record which one each job used.
ALTER TABLE scrape_jobs ADD COLUMN source TEXT NOT NULL DEFAULT 'letterboxd_rss';

-- Not every source knows a film's Letterboxd slug. UNIQUE still applies to
-- non-null values; Postgres allows any number of NULLs.
ALTER TABLE films ALTER COLUMN letterboxd_slug DROP NOT NULL;
