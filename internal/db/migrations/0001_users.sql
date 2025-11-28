CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    email         TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    name          TEXT    NOT NULL,
    password_hash TEXT    NOT NULL,
    created_at    TEXT    NOT NULL
);

CREATE TABLE sessions (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TEXT    NOT NULL,
    expires_at TEXT    NOT NULL
);

CREATE INDEX sessions_user_id ON sessions (user_id);
