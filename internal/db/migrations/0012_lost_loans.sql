-- A loan can end with the book reported lost instead of returned.
ALTER TABLE loans ADD COLUMN lost INTEGER NOT NULL DEFAULT 0;
