-- Titles sort without a leading "The", "A" or "An". The app keeps the
-- column up to date; this fills it in for titles that already exist.
ALTER TABLE titles ADD COLUMN sort_title TEXT NOT NULL DEFAULT '';

UPDATE titles SET sort_title = lower(trim(
    CASE
        WHEN lower(title) LIKE 'the %' THEN substr(trim(title), 5)
        WHEN lower(title) LIKE 'an %' THEN substr(trim(title), 4)
        WHEN lower(title) LIKE 'a %' THEN substr(trim(title), 3)
        ELSE title
    END));

CREATE INDEX titles_sort ON titles (sort_title);
