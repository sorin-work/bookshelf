-- Notes librarians keep about a member: preferred contact times, which
-- class a pupil is in, and so on. Members never see them.
ALTER TABLE users ADD COLUMN notes TEXT NOT NULL DEFAULT '';
