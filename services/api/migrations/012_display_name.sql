-- What other people see. Letterboxd users used to be shown by handle, but
-- quiz users have no handle.
ALTER TABLE users ADD COLUMN display_name TEXT CHECK (char_length(display_name) <= 40);
