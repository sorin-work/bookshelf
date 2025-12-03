CREATE TABLE titles (
    id          INTEGER PRIMARY KEY,
    isbn        TEXT    UNIQUE,
    title       TEXT    NOT NULL,
    subtitle    TEXT    NOT NULL DEFAULT '',
    publisher   TEXT    NOT NULL DEFAULT '',
    year        INTEGER,
    description TEXT    NOT NULL DEFAULT '',
    cover_url   TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL
);

CREATE TABLE authors (
    id   INTEGER PRIMARY KEY,
    name TEXT    NOT NULL UNIQUE COLLATE NOCASE
);

CREATE TABLE title_authors (
    title_id  INTEGER NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    author_id INTEGER NOT NULL REFERENCES authors (id),
    position  INTEGER NOT NULL,
    PRIMARY KEY (title_id, author_id)
);

CREATE INDEX title_authors_author ON title_authors (author_id);

CREATE TABLE subjects (
    id   INTEGER PRIMARY KEY,
    name TEXT    NOT NULL UNIQUE COLLATE NOCASE
);

CREATE TABLE title_subjects (
    title_id   INTEGER NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    subject_id INTEGER NOT NULL REFERENCES subjects (id),
    PRIMARY KEY (title_id, subject_id)
);

CREATE INDEX title_subjects_subject ON title_subjects (subject_id);
