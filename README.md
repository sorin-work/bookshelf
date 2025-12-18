# bookshelf

A small lending system for community and school libraries and book clubs.
Librarians keep a catalogue of titles and their physical copies, register
members with library cards and check books out and in at the desk. Members
can look things up in the catalogue and see what they have on loan.

## Running it

With Docker:

```sh
docker compose up --build
```

Then open http://localhost:4409. The first account you create becomes the
librarian.

Without Docker you need Go 1.24 or newer:

```sh
go run ./cmd/bookshelf
```

The database is a single SQLite file (`bookshelf.db` in the current
directory unless `DB_PATH` says otherwise). Migrations run on start.

## Tests

```sh
go test ./...
```
