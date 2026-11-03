-- The workers move a scrape job through 'embedding' after enrichment, but the
-- original CHECK constraint never allowed it, so the final enrichment update
-- was rejected and jobs stalled at 'enriching'.
ALTER TABLE scrape_jobs DROP CONSTRAINT scrape_jobs_status_check;
ALTER TABLE scrape_jobs ADD CONSTRAINT scrape_jobs_status_check
  CHECK (status IN ('queued','scraping','enriching','embedding','vectorising','generating','complete','failed'));
