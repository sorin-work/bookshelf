# Roadmap

Things I might add to bookshelf. Version 1.0.0 covers the whole loop for a small
library (catalogue, copies, members and cards, the desk, renewals, holds, fines,
reports), so nothing here is promised or scheduled. These are ideas, written down
with enough detail that I (or anyone else) can pick one up later without starting
from zero. Items may change, move between groups or be dropped.

What the app does today is in the [README](README.md), and the lending rules are in
[docs/lending-rules.md](docs/lending-rules.md).

- **Next**: small or well understood. I would start with these.
- **Later**: worth doing, but bigger or waiting on a decision.
- **Maybe**: not sure yet that they are worth the extra complexity.

Sizes are rough: **S** is an evening or two, **M** a few evenings up to a couple of
weekends, **L** several weeks of spare time.

Each item is meant to fit in one pull request and to be covered by the test suite.
There are three levels of tests and no browser tests. `internal/domain` holds the
lending rules with no database, tested table by table; `internal/store` tests run the
SQL against a temporary SQLite file; `internal/web` tests start the real router with
`httptest` against a fresh database. So a rule worth testing (a date, a fine, a queue
decision) belongs in `internal/domain`, the SQL and its transactions in
`internal/store`, and handlers stay thin. Schema changes are a new numbered file in
`internal/db/migrations` (the next one is `0014`). Paths below are relative to
`internal/` unless they start with `cmd/` or `docs/`.

When an item ships, its row in the Overview says `Shipped` in the Group column, its
section moves under a `## Shipped` heading at the end with one paragraph on what was
built and what was left out, and the same pull request updates the README (features,
configuration table), `docs/lending-rules.md` when a rule changes, and adds an entry
under a `## Unreleased` heading at the top of `CHANGELOG.md`.

"Depends on" lists real prerequisites only. Items that merely touch the same file are
not listed there; the bigger overlaps are mentioned in the items.

## Overview

| #   | Item                                    | Group | Area      | Size | Depends on |
| --- | --------------------------------------- | ----- | --------- | ---- | ---------- |
| 1   | Serve htmx from the binary              | Next  | ops       | S    | -          |
| 2   | Throttling that cannot lock people out  | Next  | security  | S    | -          |
| 3   | Scheduled backups and safe upgrades     | Next  | ops       | S    | -          |
| 4   | Paying part of a fine                   | Next  | fines     | S    | -          |
| 5   | Emails for holds and due dates          | Next  | members   | M    | -          |
| 6   | Password reset by email                 | Later | accounts  | S    | 5          |
| 7   | Confirming email addresses at sign up   | Later | accounts  | S    | 5          |
| 8   | Item types and member groups            | Later | lending   | L    | -          |
| 9   | Pausing a hold                          | Later | holds     | S    | -          |
| 10  | Stocktake                               | Later | copies    | M    | -          |
| 11  | Barcode and spine labels                | Later | copies    | M    | -          |
| 12  | Covers stored by the library            | Later | catalogue | M    | -          |
| 13  | Better catalogue search                 | Later | catalogue | M    | -          |
| 14  | Scanning with a phone camera            | Later | desk      | M    | -          |
| 15  | A desk role for volunteers              | Later | accounts  | M    | -          |
| 16  | A log of money and settings changes     | Later | ops       | M    | -          |
| 17  | Borrowing history: keep it or forget it | Later | privacy   | S    | -          |
| 18  | Purchase suggestions from members       | Later | members   | S    | -          |
| 19  | Reading lists and displays              | Later | catalogue | M    | -          |
| 20  | New arrivals                            | Later | catalogue | S    | -          |
| 21  | Accessibility pass                      | Later | ui        | M    | -          |
| 22  | Other languages                         | Later | ui        | M    | -          |
| 23  | MARC import and export                  | Later | catalogue | M    | -          |
| 24  | Replacement prices on copies            | Later | fines     | S    | -          |
| 25  | Health and metrics for running it       | Later | ops       | S    | -          |
| 26  | Browser tests for the desk              | Later | tests     | M    | 1          |
| 27  | Signed-in devices                       | Later | accounts  | S    | -          |
| 28  | Due dates in the member's calendar      | Later | members   | S    | -          |
| 29  | Importing members from a list           | Later | members   | S    | -          |
| 30  | Branches and transfers                  | Maybe | lending   | L    | 8          |
| 31  | Self-checkout                           | Maybe | desk      | M    | 14         |
| 32  | Sets for reading groups                 | Maybe | lending   | M    | 8          |
| 33  | Continuous copies of the database       | Maybe | ops       | S    | 3          |
| 34  | Two-step sign-in for librarians         | Maybe | accounts  | M    | 27         |

## Next

### 1. Serve htmx from the binary

The README says one Go binary serves everything, but `templates/layout.html` loads
htmx from `https://unpkg.com/htmx.org@2.0.11`. It has an integrity hash, so a
tampered file is refused, but three things still go wrong. A school library on a
filtered network, or a village hall with no internet, gets a desk where scanning
does a full page load (the forms still work, the "clear the barcode and focus"
behaviour in `static/app.js` does not). unpkg sees every page view of every member.
And the content security policy has to allow `https://unpkg.com` for scripts.

- Copy htmx 2.0.11 into `web/static/` (it is embedded with the rest), point
  the layout at it, and drop unpkg from `script-src` in
  `web/middleware.go`, which becomes `'self'` only.
- Keep the version in the file name or a comment and in the README's architecture
  section, so updating it is deliberate.
- Static files are served with `max-age=3600` (`web/render.go`). Put the version in
  the file name (`htmx-2.0.11.min.js`) so an upgrade is not served stale for an
  hour.
- Tricky: the integrity attribute is no longer needed for a same-origin file; leave
  it anyway as a check that the file is the one intended.
- The troubleshooting note about covers needing internet stays until item 12.
- Tests: a handler test that the layout references only same-origin scripts and that
  the htmx file is served with a JavaScript content type; the CSP test
  checks the new `script-src`.

### 2. Throttling that cannot lock people out

Failed logins are limited per email (`web/ratelimit.go`, keyed by `limiterKey(email)`
only). That slows guessing one account, but it also means anyone who knows the
librarian's email can keep them out of the desk by failing ten times every quarter
of an hour, from anywhere. And a guesser who tries one password against many emails
is not slowed at all. The limiter lives in memory, which is fine for one process.

- Key failures by email and client address together for the lock, so a stranger's
  failures do not lock the real user out from the library's own computer, and add a
  separate, looser limit per client address across all emails (say 50 an hour) for
  spraying.
- The client address: `r.RemoteAddr`, or behind a proxy the last entry of
  `X-Forwarded-For` added by it, with a `TRUSTED_PROXIES` setting (empty means never
  read the header, so it cannot be forged on a bare install).
- Sign up (when `ALLOW_SIGNUP=true`) gets a per-address limit too; today it has
  none, so a script can create accounts, and each one gets a library card number.
- Tricky: a school's whole network can be one address behind NAT. Keep the
  per-address numbers generous and log when they trigger, so a librarian can raise
  them.
- Tests: in `web/auth_test.go`: failures from another address do not block the user,
  spraying across emails is limited, the proxy header is ignored without
  `TRUSTED_PROXIES`.

### 3. Scheduled backups and safe upgrades

`bookshelf backup FILE` makes a consistent copy, which is good. Nobody runs it unless
they remember to, and nothing removes old copies. The other half is missing: no
restore command, and no check on the way back. `db.Migrate` applies the migrations
it does not know as applied, but says nothing when the database has migrations the
binary does not know, which is what happens when someone rolls back to an older image
after an upgrade. The older binary then runs against a schema it was never written
for.

- `BACKUP_DIR` and `BACKUP_KEEP` settings. When `BACKUP_DIR` is set, the hourly
  housekeeping in `jobs/jobs.go` makes one backup a day (named by date, skipped if
  today's exists) and deletes the oldest beyond `BACKUP_KEEP`. The compose file sets
  `BACKUP_DIR=/data/backups`.
- `bookshelf restore FILE`: refuses to run while the app holds the database (check
  with a write lock), checks that the file is a bookshelf database (has
  `schema_migrations`) and not newer than the binary, then copies it into place.
- `Migrate` refuses to start when `schema_migrations` has a version higher than the
  highest embedded one, with a message that names both and says to restore a backup
  or run the newer version.
- Docs: a "Backups and upgrades" section in the README: where the daily copies are,
  copying them off the machine (a volume on the same disk is not a backup), restore
  steps, and that an upgrade is "pull, up --build" with migrations on start.
- Tricky: `VACUUM INTO` takes a read transaction for the whole copy. For a library
  database that is seconds; fine at night, but run it at a quiet hour rather than on
  start.
- Tests: `db/backup_test.go` for retention with a fixed clock; `db/migrate_test.go`
  for the newer-database refusal; a restore round trip.

### 4. Paying part of a fine

A fine is open, paid or waived (`store.ResolveFine`), so a member who owes 7.50 and
has 5.00 on them cannot pay anything: the librarian either waives the rest, writes
it down on paper, or turns them away. Borrowing stays blocked at `BlockAt` either
way.

- A `payments` table (fine, amount in cents, taken by, at, method note) and a "Take
  payment" form on the member page with an amount. A payment is applied to the
  oldest open fines first, closing each one it covers in full and leaving a remainder
  on the next.
- `finesOwed` becomes the sum of open fines minus payments on them; the fine itself
  keeps its original amount, so the history reads correctly.
- The allocation (oldest first, partial on the last) is a pure function in
  `domain/` with table tests.
- Tricky: waiving a fine that is partly paid waives only the rest.
- The monthly report has no money in it today. With payments it can show what was
  taken each month, counted by payment date, so a fine paid in three instalments
  shows up in three months.
- Tests: domain tests for allocation; store tests for the owed sum and blocking at
  `BlockAt`; a handler test for the form.

### 5. Emails for holds and due dates

The app sends no email at all. When a held book comes back, the desk is told to put
it on the hold shelf, and the member finds out only if they look at their account;
the pickup clock (7 days) is running meanwhile, and the hold expires unseen. The
overdue list's "reminder" is a `mailto:` link (`web/overdue.go`) that a librarian
clicks one member at a time. Nobody gets "due in three days".

- Optional SMTP settings (`SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`,
  `SMTP_PASSWORD`, `MAIL_FROM`); without them nothing is sent and the `mailto:` links
  stay. Sending through `net/smtp` with STARTTLS; no library needed.
- An `outbox` table (to, subject, body, kind, related id, attempts, next attempt,
  sent at, last error). Events write a row inside the transaction that causes them:
  a copy put on the hold shelf (in `shelveCopy`), a hold expired or cancelled because
  it can never be filled. The hourly housekeeping adds due-soon (N days before, a
  setting) and overdue (on day 1 and day 7 after the due date) reminders, once per
  loan per kind.
- The housekeeping sends due rows and retries failures with growing gaps; a mail
  server that is down for an hour loses nothing. Hourly is slow for "your hold is
  ready"; send that kind straight after the commit as well, with the outbox as the
  retry.
- Members can turn reminder kinds off on their account page; hold notices are always
  sent because they cost the member their place otherwise.
- Email bodies as `text/template` files embedded like the HTML ones, plain text
  only, with the library name and its opening days from the policy.
- Tricky: due dates and "three days before" are library dates (`domain.Date` in
  `LIBRARY_TIMEZONE`), and the job runs hourly in UTC. Work out "today" the same way
  the desk does and claim each reminder with a unique key so overlapping runs send it
  once.
- Tricky: members created at the desk may have no email; skip them and show it on the
  member page.
- A mailpit service in the compose file (on a free port) for development.
- Tests: store tests that shelving writes one outbox row; job tests with a fixed
  clock for due-soon and overdue reminders sent once; a fake SMTP server for the
  sender, including a failure that is retried.

## Later

### 6. Password reset by email

A member who forgets their password asks a librarian, who sets a new one
(`POST /members/{id}/password`) and has to pass it on somehow. A librarian who forgets
theirs, in a library with one librarian, has no way back in except editing the
database.

- "Forgot password" on the login page when SMTP is configured: a random token
  (stored hashed, like sessions), valid for an hour, sent through the outbox from
  item 5. Using it sets the password and signs out every other session, as a password
  change already does.
- The answer never says whether the email exists, and requests are limited per
  address and email (item 2's limiter).
- Without SMTP, `bookshelf reset-password EMAIL` on the command line prints a
  one-time link, which gives the lone librarian a way back.
- Tests: the link works once, expires, is refused after a password change.

### 7. Confirming email addresses at sign up

With `ALLOW_SIGNUP=true` (the default), anyone can create an account with any email
and gets a library card. Item 5 would then send reminders to an address that never
asked for them.

- New online accounts get `email_verified_at` null and a confirmation email; until
  confirmed they can search and place holds but cannot be emailed reminders and are
  marked on the member page.
- Accounts made at the desk are trusted (the librarian saw the person), and existing
  accounts are marked verified in the migration.
- Undecided: whether unconfirmed accounts may borrow. They cannot borrow online
  anyway (borrowing happens at the desk with a card), so I would not block it.
- Tests: confirmation link, unverified accounts skipped by reminders.

### 8. Item types and member groups

There is one set of rules for everything. Real libraries lend DVDs for a week,
magazines not at all, and let children borrow fewer items with no fines. Today the
only way is to tell the librarian to remember.

- Item types (book, DVD, magazine, reference, plus what the library adds) with their
  own loan days, renewals, fine per day and cap, and a "not for loan" flag. Each copy
  has a type; the title's default type fills new copies.
- Member groups (adult, child, staff) with their own loan limit, hold limit, block
  amount and a "no fines" flag. Each member has a group.
- A rule for a checkout is the item type's values with the member group's limits on
  top. `domain.Policy` becomes the library-wide part (closing days, pickup days,
  currency) plus a small table, and the functions in `domain/loan.go` and
  `domain/policy.go` take the rule for that loan instead of the whole policy.
- Loans store the rule they were made under (loan days, fine per day, cap), so
  changing a type's settings does not change fines for books already out. Same idea
  as the loan period being fixed at checkout today.
- The settings page grows a table per kind; the default type and group carry
  today's values, so an existing library sees no change.
- Tricky: "how many items out" with mixed limits. Keep one total limit from the
  member group and optional per-type limits ("at most 2 DVDs").
- Tricky: holds. The estimated wait on the title page uses the loan period; with
  types it uses the type of the copies.
- Large because every lending rule touches it. Worth it: it is the most common
  configuration question in library software.
- Tests: domain tables for the combined rule; store tests for checkout refused on a
  not-for-loan copy and fines worked out with the rule stored on the loan.

### 9. Pausing a hold

A member going on holiday for three weeks either cancels their holds and loses
their place, or has a copy wait on the hold shelf for seven days and expire.

- "Pause until" on a waiting hold: the hold keeps its place in the queue but is
  skipped when a copy comes back (`waitingQueue` filters it), and wakes up on the
  date.
- A ready hold cannot be paused; the copy is already waiting.
- The rule is a small change in `domain/hold.go` (which hold gets the copy) with
  table tests; the queue position shown on the account page says "paused".
- Tricky: renewals are refused while someone is waiting; a paused hold should not
  block renewals, or one person on holiday stops everyone renewing.
- Tests: domain tests for skipping and waking; store test that a return passes the
  paused member over; renewal allowed with only paused holds.

### 10. Stocktake

Copies can be marked lost when someone notices. A yearly stocktake (scan everything
on the shelves, find what is missing) is how libraries actually notice, and doing it
on paper against a printed list is the job everyone dreads.

- A stocktake session: started by a librarian, optionally limited to shelves.
  Scanning barcodes at a stocktake page (the desk's scanning code, without loans)
  records each one as seen.
- At the end: copies expected on those shelves (status available) and not seen, with
  "mark as lost" in bulk; copies seen that the system thought were on loan (a return
  never checked in), with "check in"; barcodes not in the catalogue.
- Tricky: the library stays open during a stocktake. A copy checked out after the
  session started is not missing; compare against status at the end, not the start.
- Tests: store tests for the three lists; a checkout during the session.

### 11. Barcode and spine labels

New copies get a suggested barcode, but there is no way to print it. Libraries
either buy pre-printed labels and type the numbers in, or write them by hand.

- A label sheet page for a selection of copies (new since a date, a title, a shelf):
  printable HTML sized for common A4 label sheets (the sizes as a small table), each
  label with the barcode as Code 128 and the shelf mark. Library cards already have a
  print page; this reuses that approach.
- Code 128 drawn as inline SVG by a small encoder in `domain/` (no dependency); the
  encoding is short and has public test vectors.
- Spine labels with the shelf mark and the start of the author's surname, same sheet
  layout.
- Tests: the encoder against known outputs; the page renders the selected copies.

### 12. Covers stored by the library

Covers are image urls on the title, loaded by the browser from Open Library or
wherever the librarian pointed them. That needs internet on the reader's side (the
README's troubleshooting mentions it), tells the image host who looks at which
title, and is why the content security policy allows images from any `https:` site.

- When a title is saved with a cover url, the server downloads the image once, checks
  it is a real JPEG, PNG or WebP under a size limit, scales it to a fixed width with
  `image/draw`, and stores it in a `covers` table (or files under `DATA_DIR/covers`).
  Pages use `/covers/{title id}`.
- A librarian can also upload a cover from a photo of the book.
- Existing titles get their covers fetched by the housekeeping job, a few at a time.
- Then `img-src` becomes `'self' data:`.
- Tricky: fetching urls a librarian typed. http and https only, refuse private and
  loopback addresses (resolve once and connect to the checked address), a time limit
  and a size limit. The Open Library lookup already talks to one fixed host; this is
  any host.
- Tricky: the database grows. Covers at 300 pixels wide are 20 to 40 KB each, so a
  10,000 title library adds a few hundred megabytes to the backups. Files on disk
  next to the database keep backups small but need copying too. I lean towards the
  database for one-file simplicity.
- Tests: decoding and scaling a fixture image; refusing a private address and a
  non-image; the `/covers` handler with caching headers.

### 13. Better catalogue search

Search is `LIKE '%term%'` on title, subtitle, ISBN and author names
(`store.ListTitles`). It reads every row for every keystroke (a leading `%` cannot
use an index), finds "war" inside "Stewart", needs the exact accents (a search for
"Bronte" misses "Brontë", "Creanga" misses "Creangă"), and
sorts by title rather than by how well it matches. Descriptions and subjects are not
searched.

- SQLite FTS5 (available in modernc.org/sqlite): a `titles_fts` virtual table over
  title, subtitle, authors, subjects and description, with the `unicode61` tokenizer
  and `remove_diacritics 2`, kept in sync by the store functions that write titles
  (or triggers). Prefix queries (`term*`) for search as you type; `bm25()` with title
  and author weighted above description for ordering.
- ISBN search stays exact on the `isbn` column, after normalising what was typed
  with the existing ISBN code.
- Filters (subject, author, available only) stay as they are and combine with the
  FTS match.
- Tricky: the migration builds the index for existing titles; for big catalogues
  that is a few seconds on start, which is fine once.
- Tricky: FTS5 query syntax. User input must be quoted per term, or a stray `"` or
  `-` becomes a syntax error.
- Tests: accents, prefix matching, ranking (title match before description match),
  odd characters in the query, sync after an edit.

### 14. Scanning with a phone camera

The desk assumes a USB barcode scanner (it types into the field and presses Enter).
Small libraries and book clubs often have none, and typing 13 digit barcodes is
slow and error prone. Adding a title by ISBN has the same problem.

- A "scan with camera" button next to the barcode, card and ISBN fields that opens
  the camera and fills the field when a code is read, using the browser's
  `BarcodeDetector` where it exists (Chrome on Android, Safari is getting it).
- Where it does not exist, the button is hidden. A JavaScript decoder library would
  cover every browser, at the cost of a vendored dependency; I would start without
  it.
- Tricky: the camera needs https (or localhost). Say so next to the button when the
  page is on plain http.
- Tricky: the CSP may need `media-src` or nothing at all; check, and keep `'self'`.
- Tests: the desk still works without the button (it is progressive); a small test
  for the JavaScript is not possible with the current setup, so keep the script
  short and the field contract (fill, then submit) the same as a USB scanner's.

### 15. A desk role for volunteers

There are two roles, librarian and member. Many community and school libraries run
on volunteers at the desk. Giving them the librarian role gives them settings,
member data, fines, role changes and the delete button on titles.

- A `desk` role: the desk (check out, check in, renew), placing holds for members,
  looking up a member by card, taking payments. Not: settings, roles, editing or
  anonymising members, deleting titles, exports, waiving fines.
- `requireLibrarian` in `web/routes.go` splits into checks by permission; the route
  table says which permission each route needs, so it can be reviewed in one place.
- Tricky: the first account is the librarian; at least one librarian must always
  remain, as today.
- Tests: a table over every route and role, asserting 403 or 200, so a new route
  cannot be added without deciding.

### 16. A log of money and settings changes

Fines are waived, payments taken, lending rules changed, members' roles changed. The
fines table keeps who resolved a fine and when, nothing else is recorded. With more
than one person at the desk (item 15), "who waived 20 euros of fines on Tuesday" is a
reasonable question with no answer.

- An `audit_log` table (who, what, which record, before and after as JSON, when),
  written in the same transaction by the store functions for: fines added, paid and
  waived, payments (item 4), settings saved, roles changed, members anonymised,
  copies marked lost or withdrawn, titles deleted.
- A page for librarians, filterable by member, kind and date, and a CSV download.
- Tricky: anonymising a member must also remove their personal details from the
  log entries about them, or the log keeps what the anonymise removed.
- Tests: each action writes one entry; anonymise scrubs them.

### 17. Borrowing history: keep it or forget it

Every loan is kept forever with the member's id, and the member's history page lists
them. Libraries traditionally keep as little reading history as they can (who read
what is sensitive), while some members like a record of what they read. Today the
only option is anonymising a closed account.

- A library setting: keep history (today), or detach returned loans from the member
  after N days (set `user_id` to a shared "former loans" placeholder, keep copy,
  dates and fines), so statistics still add up.
- A member setting, when the library keeps history: "don't keep my reading history".
- The housekeeping job does the detaching.
- Tricky: loans with an open fine stay linked until the fine is closed.
- Tricky: the monthly report counts loans by member group (item 8) or unique
  borrowers; detached loans cannot be counted by person. Decide what the report says
  for those.
- Tests: detaching after N days with a fixed clock; loans with open fines kept;
  member data download reflects the setting.

### 18. Purchase suggestions from members

Members ask for books the library does not have, at the desk, by email or on a
scrap of paper.

- "Suggest a book" for signed-in members: ISBN or title and author, filled from Open
  Library by ISBN when lookups are on. A list for librarians with "ordered",
  "declined" and "added" (linking the new title), and the member sees the state on
  their account page.
- When a suggestion is added to the catalogue, offer to place a hold for the member
  who asked.
- Tests: create, change state, the hold offer.

### 19. Reading lists and displays

Librarians make lists: summer reading for year 6, the book club's picks, a display
for a theme month. Subjects are the closest thing today and they describe the book,
not the occasion.

- Lists with a name, description, an ordered set of titles and a public or private
  flag. Public lists appear on the catalogue page and can be linked to; each title
  shows availability as the catalogue does.
- Printable list page, like the due date slips.
- Tests: create and reorder, public visibility, availability on the list.

### 20. New arrivals

Members have no way to see what came in this month except browsing.

- A "new on the shelves" page: titles whose first copy was added in the last N days
  (a setting), newest first, with an Atom feed for people who use feed readers.
- Tricky: a big import (CSV) of an existing collection makes everything "new". Let
  the import mark titles as "not new" (an option on the import page).
- Tests: the window, the import option, the feed is valid Atom.

### 21. Accessibility pass

Pages are server-rendered forms, which is a good start. The desk is the part that
matters most: a librarian scanning all day, possibly with a screen reader or high
zoom, and htmx swapping results into the page.

- The desk's result area as an `aria-live` region, so "Checked out, due 14 March" and
  "Card expired" are announced after a scan, and errors take focus.
- Status shown by colour (overdue, on hold shelf) also as text.
- Check headings, labels and focus outlines across pages, and that the printable
  slips and cards print in high contrast.
- Tests: handler tests for the live region and text labels on the desk responses.

### 22. Other languages

Everything is English. The defaults hint at a library outside the English-speaking
world (euro, Sunday closure), and school libraries need the language of their
pupils.

- Message catalogues for templates and handler messages (`golang.org/x/text/message`
  or a small map per language embedded in the binary), a `LIBRARY_LANGUAGE`
  setting, and dates through a formatter that knows month and day names.
- One extra language shipped with the item, so the machinery is proved.
- Tricky: plurals ("1 item", "2 items", and languages with more forms).
- Tricky: sorting titles without a leading article (`0013_sort_title.sql`) is
  English specific ("The", "A", "An"); the article list becomes per language.
- Tests: a page rendered in the second language; plural forms; sort keys per
  language.

### 23. MARC import and export

Libraries moving from other software, or getting records from a union catalogue,
have MARC 21 records. The CSV import covers spreadsheets but not those.

- Import MARC 21 (ISO 2709 binary) and MARCXML: title (245), authors (100, 700),
  ISBN (020), publisher and year (264 or 260), subjects (650), description (520), and
  copies from holdings fields (852 or 952, which other systems use for barcodes and
  shelves). Unknown fields are ignored and counted.
- Export the catalogue as MARCXML with the same fields, so leaving is as easy as
  arriving.
- A parser in a new `internal/marc` package with no dependencies; the format is
  simple at the record level and has odd corners (directory lengths, character
  encoding flag in the leader).
- Tricky: MARC-8 encoding in older records. Support UTF-8 only at first and say so
  when the leader says MARC-8.
- Tests: fixture records from public sources, round trip of an export.

### 24. Replacement prices on copies

A book reported lost ends the loan without a late fine, and the librarian "can add a
replacement charge instead", typing the amount each time and looking up the price
somewhere.

- An optional `replacement_cents` on copies (and a default on the title, and per
  item type with item 8). Declaring a loan lost offers to charge that amount, with
  the note filled in.
- If the copy turns up and is checked in later, offer to refund (waive) the
  replacement charge, which libraries usually do.
- Tests: charge offered with the price; check in of a lost copy offers the waiver.

### 25. Health and metrics for running it

`/healthz` answers for the process and logs are structured. Nothing tells a librarian
that the hourly job stopped, that emails (item 5) fail, or that backups (item 3)
have not been made for a week.

- `/healthz` reports the last housekeeping run, last backup and failing emails, with
  `degraded` when one of them is late; the HTTP status stays 200 so the container is
  not restarted for it.
- A short status box on the librarian dashboard with the same facts: people running
  this will not read logs.
- Optional `/metrics` in Prometheus text format behind a token (written by hand: a
  few counters for checkouts, returns, requests by status, job runs).
- Tests: health output with a fixed clock and an old backup.

### 26. Browser tests for the desk

The desk is htmx: scan, swap, clear the field, focus again (`static/app.js`). The
handler tests check the HTML that comes back but not that the page does the right
thing with it, and a broken `hx-target` would pass every test.

- A few tests with a headless browser (chromedp, which drives Chrome from Go with no
  Node toolchain): check out two books on one card, check one in to the hold shelf,
  renew from the account page, search as you type.
- A separate CI job, so `go test ./...` stays fast; skipped when no Chrome is found.
- Depends on item 1: the tests must not reach unpkg.
- Tests: these are the tests. Keep the set small.

### 27. Signed-in devices

Sessions are rows in `sessions` with a hash of the token. A password change signs out
everywhere else (`store/users.go`), but there is no way to see where you are signed
in, or to sign out the desk computer from home, short of changing the password.

- Record a user agent summary and the last time seen (updated at most every few
  minutes) on sessions; an account page lists them with "sign out" per row and "sign
  out everywhere else".
- Librarians can sign a member out everywhere from the member page (for a lost
  phone).
- Deactivating or anonymising a member deletes their sessions; check that it does
  today and add it where it does not.
- Tests: list, revoke one, revoke the rest, deactivation removes sessions.

### 28. Due dates in the member's calendar

Members check their due dates on the account page. A calendar subscription would put
them on their phone with the phone's own reminders, without any email.

- A private iCalendar feed per member (`/account/{token}.ics`, token stored hashed,
  can be reset) with an all-day event per open loan on its due date and one for each
  hold waiting on the shelf with its pickup deadline.
- Events are built by a small writer in `domain/` (the format is a few lines per
  event; escape commas, semicolons and newlines, fold long lines).
- Tests: the writer's escaping and folding; the feed shows only that member's loans;
  a reset token stops the old url.

### 29. Importing members from a list

A school library starts the year with a class list of three hundred pupils. Each one
is registered by hand at the desk today. At the end of the year the leavers are
deactivated by hand too.

- Import members from CSV (name, email, phone, group from item 8 if present, card
  number or generate one), with a preview of what will be created, and errors per
  line, as the title import does (`web/import.go`).
- Bulk actions on the members list: renew cards to a date, deactivate the selected,
  for the end of a school year.
- Tricky: duplicates. Match on email when present, otherwise on name and card
  number, and show matches in the preview rather than guessing.
- Tests: preview and import, duplicate matching, bulk card renewal.

## Maybe

### 30. Branches and transfers

Some community libraries are two rooms in two villages sharing one collection. The
app has one location.

- Branches with their own closing days; copies belong to a home branch and have a
  current location. Holds name a pickup branch; a returned copy for a hold at
  another branch goes "in transit" until checked in there.
- Depends on item 8 because loan rules per branch come up immediately.
- Tricky: the copy status set (`available`, `on_loan`, `on_hold_shelf`, ...) gets an
  `in_transit`, and every place that counts available copies needs to know which
  branch it is asking about. That touches holds, the catalogue and the desk, which
  is why this is Maybe.

### 31. Self-checkout

A kiosk mode where members scan their own card and books, for libraries with one
volunteer and a queue.

- A locked-down page (signed in as a special kiosk account) that only allows
  checkout and renewal, signs the member out after each transaction, and prints or
  emails a slip.
- Depends on item 14 for a camera-only setup.
- Tricky: card numbers become the only credential at the kiosk, and they are printed
  on the card. Fine for checkout (it is what a desk does), not for anything else.
  Maybe until someone asks.

### 32. Sets for reading groups

Book clubs borrow eight copies of the same title for six weeks, and the library lends
them as a set to the club's organiser. Today that is eight separate loans that each
fall due and get renewed separately.

- A set is a group of copies lent as one loan to one member, with its own loan
  period from item 8, checked in together (with a warning if a copy is missing).
- Tricky: a set loan is several copies on one loan row, or several loan rows tied
  together. The second keeps fines and the open loan index (`loans_open_copy`) as
  they are; I would do that.

### 33. Continuous copies of the database

Daily backups (item 3) can lose a day of desk work. Streaming the SQLite write-ahead
log to another place (Litestream does this, self-hostable, to a disk or any S3
compatible store) loses seconds.

- No code in the app: a documented compose profile that runs Litestream next to the
  app against the same volume, and a restore procedure.
- Tricky: `bookshelf restore` from item 3 must not fight with it; document stopping
  Litestream first.

### 34. Two-step sign-in for librarians

Librarian accounts see members' names, addresses, phone numbers and reading history.
Email and password is the only protection, and library computers are shared.

- TOTP with recovery codes for librarians (optional for members), written on the
  standard library (`crypto/hmac`); a library-wide setting to require it for the
  librarian role.
- Depends on item 27 so that turning it on can sign out existing sessions.
- Tests: RFC 6238 test vectors, sign-in with and without a code, recovery codes used
  once.
