# bookshelf

A small lending system for community and school libraries and book clubs.
Librarians keep a catalogue of titles and their physical copies, register
members with library cards and check books out and in at the desk. Members
search the catalogue, place holds on books that are out and keep an eye on
their own loans.

## What it does

- **Catalogue**: titles with ISBN (10 or 13 digits, stored as ISBN-13),
  authors, subjects, publisher, year and a cover image. Search as you type,
  browse by subject or author. New titles can be filled in from Open
  Library by ISBN, or imported in bulk from a CSV file.
- **Copies**: each physical copy has its own barcode and shelf; copies can
  be marked lost or withdrawn.
- **Members and cards**: anyone can sign up online, or a librarian registers
  them at the desk. Every account gets a library card number with a check
  digit and an expiry date.
- **Circulation desk**: scan a card and a barcode to check out, scan a
  barcode to check in. Due dates skip the days the library is closed.
- **Renewals**: limited per loan, refused for overdue items and while
  someone is waiting for the title.
- **Holds**: when every copy is out members join a first-come queue. A
  returned copy goes to the hold shelf for the next person and waits a set
  number of days before passing on.
- **Fines**: a daily fine for late returns with a cap and optional grace
  days. Members who owe too much can't borrow until they pay.
- **Dashboard and overdue list** for librarians.
- **Lending rules** (loan period, renewals, limits, holds, fines, closing
  days) are edited on the settings page.

The first account created becomes the librarian.

## Running it

With Docker:

```sh
docker compose up --build
```

Then open http://localhost:4409. The compose file loads demo data:
log in as `demo@example.com` / `demo-password` (librarian) or
`member@example.com` / `demo-password` (member).

Without Docker you need Go 1.25 or newer:

```sh
SEED_DEMO=true go run ./cmd/bookshelf
```

The database is a single SQLite file (`bookshelf.db` in the current
directory unless `DB_PATH` says otherwise). Migrations run on start.

## Configuration

| Variable           | Default        | Meaning                                              |
| ------------------ | -------------- | ---------------------------------------------------- |
| `ADDR`             | `:4409`        | Address to listen on                                 |
| `DB_PATH`          | `bookshelf.db` | SQLite file                                          |
| `LIBRARY_TIMEZONE` | `UTC`          | Time zone due dates are worked out in                |
| `SECURE_COOKIES`   | `false`        | Mark cookies Secure; turn on when served over https  |
| `SESSION_TTL`      | `336h`         | How long a login lasts                               |
| `SEED_DEMO`        | `false`        | Create the demo accounts and library on first start  |
| `OPENLIBRARY_URL`  | `https://openlibrary.org` | Where ISBN lookups go; `off` disables them |
| `LOG_LEVEL`        | `info`         | `debug`, `info`, `warn` or `error`                   |
| `LOG_FORMAT`       | `json`         | `json` or `text`                                     |

## Tests

```sh
go test ./...
```
