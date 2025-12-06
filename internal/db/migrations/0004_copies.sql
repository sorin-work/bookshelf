CREATE TABLE copies (
    id         INTEGER PRIMARY KEY,
    title_id   INTEGER NOT NULL REFERENCES titles (id) ON DELETE RESTRICT,
    barcode    TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    status     TEXT    NOT NULL DEFAULT 'available'
        CHECK (status IN ('available', 'on_loan', 'on_hold_shelf', 'lost', 'withdrawn')),
    shelf      TEXT    NOT NULL DEFAULT '',
    note       TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL
);

CREATE INDEX copies_title ON copies (title_id);
