ALTER TABLE users ADD COLUMN phone TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN card_number TEXT;
ALTER TABLE users ADD COLUMN card_expires TEXT;

CREATE UNIQUE INDEX users_card_number ON users (card_number);
