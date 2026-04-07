-- Members who leave are deactivated rather than deleted so their loan and
-- fine history stays intact.
ALTER TABLE users ADD COLUMN active INTEGER NOT NULL DEFAULT 1;
