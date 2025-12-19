CREATE TABLE holds (
    id         INTEGER PRIMARY KEY,
    title_id   INTEGER NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status     TEXT    NOT NULL DEFAULT 'waiting'
        CHECK (status IN ('waiting', 'ready', 'fulfilled', 'cancelled', 'expired')),
    placed_at  TEXT    NOT NULL,
    -- Set once a copy is waiting on the hold shelf.
    copy_id    INTEGER REFERENCES copies (id),
    ready_on   TEXT,
    expires_on TEXT,
    closed_at  TEXT
);

CREATE INDEX holds_queue ON holds (title_id, status, placed_at);
CREATE INDEX holds_user ON holds (user_id, status);
-- One open hold per member and title.
CREATE UNIQUE INDEX holds_open_member_title ON holds (title_id, user_id)
    WHERE status IN ('waiting', 'ready');
