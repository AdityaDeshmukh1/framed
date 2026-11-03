ALTER TABLE taste_vectors DROP COLUMN confidence;
ALTER TABLE taste_vectors DROP COLUMN film_count;
DROP TABLE film_stats;
ALTER TABLE films DROP COLUMN in_quiz_pool;
ALTER TABLE films DROP COLUMN embedding_version;
