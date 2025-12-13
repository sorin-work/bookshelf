CREATE TABLE loans (
    id             INTEGER PRIMARY KEY,
    copy_id        INTEGER NOT NULL REFERENCES copies (id),
    user_id        INTEGER NOT NULL REFERENCES users (id),
    checked_out_at TEXT    NOT NULL,
    checked_out_by INTEGER REFERENCES users (id) ON DELETE SET NULL,
    due_on         TEXT    NOT NULL,
    renewals       INTEGER NOT NULL DEFAULT 0,
    returned_at    TEXT
);

CREATE INDEX loans_user ON loans (user_id, returned_at);
CREATE INDEX loans_due ON loans (due_on) WHERE returned_at IS NULL;
-- At most one open loan per copy.
CREATE UNIQUE INDEX loans_open_copy ON loans (copy_id) WHERE returned_at IS NULL;

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
