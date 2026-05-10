ALTER TABLE user_ratings ALTER COLUMN rating DROP NOT NULL;
ALTER TABLE user_ratings DROP CONSTRAINT user_ratings_rating_check;
ALTER TABLE user_ratings ADD CONSTRAINT user_ratings_rating_check CHECK (rating >= 0.5 AND rating <= 5.0);