CREATE TABLE fines (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    loan_id      INTEGER REFERENCES loans (id) ON DELETE SET NULL,
    kind         TEXT    NOT NULL CHECK (kind IN ('late', 'charge')),
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    note         TEXT    NOT NULL DEFAULT '',
    status       TEXT    NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'paid', 'waived')),
    created_at   TEXT    NOT NULL,
    resolved_at  TEXT,
    resolved_by  INTEGER REFERENCES users (id) ON DELETE SET NULL
);

CREATE INDEX fines_user ON fines (user_id, status);
