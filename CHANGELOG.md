# Changelog

## 1.0.0

The first release considered ready for a real library.

### Catalogue
- Titles with ISBN-10 or ISBN-13 (stored as ISBN-13), authors, subjects,
  publisher, year, description and cover.
- Search as you type, browse by subject or author, filter to what's on
  the shelf; titles sort without a leading "The", "A" or "An".
- Fill in a new title from Open Library by ISBN.
- Import titles and copies from CSV and export the catalogue in the same
  format.
- Physical copies with barcodes (the next one in the series is
  suggested), shelves and notes; copies can be marked lost or withdrawn.

### Members
- Online sign up (can be switched off) or registration at the desk, with
  a library card number that has a check digit and an expiry date.
- Librarian and member roles; the first account is the librarian.
- Closing accounts, removing personal details on request, and a data
  download for members.

### Circulation
- Circulation desk for checkout, check in and renewals by scanning.
- Due dates skip weekly closing days and one-off closures.
- Renewal limits; no renewals when overdue, with an expired card or while
  someone is waiting.
- First come, first served holds with a hold shelf, pickup deadlines and
  hourly expiry; librarians can place holds for members.
- Late fines per day with a cap and grace days, manual charges, payments
  and waivers; borrowing is blocked over a set amount.
- Dashboard, overdue list (with CSV and reminder emails), monthly report,
  printable due date slips and library cards.

### Running it
- One Go binary with an embedded SQLite database, migrations and
  templates; Docker image and compose file with a volume for the data.
- `bookshelf backup FILE` for a consistent copy while running.
- Demo library with `SEED_DEMO=true`.
