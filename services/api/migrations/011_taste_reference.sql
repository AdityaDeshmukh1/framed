-- Reference vectors for taste maths. 'catalogue_mean' is the popularity-
-- weighted mean film embedding, subtracted before building taste vectors
-- (see internal/taste). It is computed over the whole catalogue by
-- cmd/catalogue, so it stays stable no matter how few users there are.
CREATE TABLE taste_reference (
    name       TEXT PRIMARY KEY,
    vector     vector(384) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
