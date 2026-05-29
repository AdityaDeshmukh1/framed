-- Track enrichment progress per scrape job.
-- films_processed now means "enrich tasks queued" (not total ratings).
-- films_enriched increments as each enrich task finishes.
-- enrichment_started_at is stamped when the first enrich task picks up.
ALTER TABLE scrape_jobs
  ADD COLUMN films_enriched       INT         NOT NULL DEFAULT 0,
  ADD COLUMN enrichment_started_at TIMESTAMPTZ;
