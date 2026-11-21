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
- **Dashboard, overdue list and monthly report** for librarians, with a
  CSV download of the overdue list and printable due date slips and
  library cards.
- **Privacy**: members can download their data; closed accounts can have
  their personal details removed while loan statistics stay intact.
- **Lending rules** (loan period, renewals, limits, holds, fines, closing
  days) are edited on the settings page.

The first account created becomes the librarian. The rules for due dates,
renewals, holds and fines are described in
[docs/lending-rules.md](docs/lending-rules.md).

## Architecture

One Go binary serves server-rendered HTML; there is no separate frontend
build. Pages are `html/template` files embedded in the binary, and
[htmx](https://htmx.org) swaps in fragments where a full page load would
get in the way: search as you type, the circulation desk, renewing a loan
or cancelling a hold in place. Everything still works as plain forms
without JavaScript.

Data lives in a single SQLite file, opened with the pure Go
[modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) driver (no
cgo). Migrations are numbered `.sql` files embedded in the binary and run
on start.

```
cmd/bookshelf          main: config, database, background jobs, HTTP server
internal/config        settings from environment variables
internal/db            opening SQLite, migrations, backups
internal/domain        lending rules with no database: ISBNs, card numbers,
                       dates, due dates, renewals, holds, fines
internal/store         SQL for every table; checkout, return, renewal and
                       holds run in transactions
internal/web           handlers, middleware, sessions, CSRF, templates,
                       static files
internal/jobs          hourly housekeeping: expiring holds and sessions
internal/seed          the demo library
internal/openlibrary   ISBN lookups
internal/version       the version shown in the footer
```

Logins are session cookies backed by a `sessions` table (only a hash of
the token is stored); passwords are hashed with bcrypt. Every form and
htmx request carries a CSRF token, and cross-site posts are also refused
by fetch metadata. An hourly job expires uncollected holds and old
sessions. Logs are structured with `log/slog`.

## Running it

With Docker:

```sh
docker compose up --build
```

Then open http://localhost:4409. The compose file loads demo data:
log in as `demo@example.com` / `demo-password` (librarian) or
`member@example.com` / `demo-password` (member).

Without Docker you need Go 1.26 or newer:

```sh
SEED_DEMO=true go run ./cmd/bookshelf
```

The database is a single SQLite file (`bookshelf.db` in the current
directory unless `DB_PATH` says otherwise). Migrations run on start.

## Backups

Everything lives in the one SQLite file. To copy it safely while the app
is running:

```sh
bookshelf backup /backups/bookshelf-$(date +%F).db
# or, with Docker:
docker compose exec app bookshelf backup /data/backup-$(date +%F).db
```

## Troubleshooting

- **"This form has expired"**: the CSRF cookie was missing or changed,
  usually because cookies were cleared or the page was open for a long
  time. Reload the page and submit again.
- **Logged out straight away behind https**: set `SECURE_COOKIES=true` so
  the browser keeps the cookies; on plain http leave it `false`.
- **Due dates a day off**: set `LIBRARY_TIMEZONE` to the library's zone,
  for example `Europe/Bucharest`.
- **Covers don't show**: covers are loaded from the address on the title
  (Open Library for looked up titles), so the browser needs internet
  access.

## Roadmap

Ideas for what could come next, with notes on how each would be built, are in
[ROADMAP.md](ROADMAP.md).

## Configuration

| Variable           | Default        | Meaning                                              |
| ------------------ | -------------- | ---------------------------------------------------- |
| `ADDR`             | `:4409`        | Address to listen on                                 |
| `DB_PATH`          | `bookshelf.db` | SQLite file                                          |
| `LIBRARY_NAME`     | `Bookshelf`    | Shown in the header and on printed slips             |
| `LIBRARY_TIMEZONE` | `UTC`          | Time zone due dates are worked out in                |
| `ALLOW_SIGNUP`     | `true`         | `false` hides online sign up                         |
| `SECURE_COOKIES`   | `false`        | Mark cookies Secure; turn on when served over https  |
| `SESSION_TTL`      | `336h`         | How long a login lasts                               |
| `SEED_DEMO`        | `false`        | Create the demo accounts and library on first start  |
| `OPENLIBRARY_URL`  | `https://openlibrary.org` | Where ISBN lookups go; `off` disables them |
| `LOG_LEVEL`        | `info`         | `debug`, `info`, `warn` or `error`                   |
| `LOG_FORMAT`       | `json`         | `json` or `text`                                     |

## Tests and linting

```sh
go test ./...            # unit tests and handler tests against a temporary SQLite file
go vet ./...
golangci-lint run ./...
```

The domain rules have table-driven tests; the handler tests start the
real router with `httptest` against a fresh database for each test.
