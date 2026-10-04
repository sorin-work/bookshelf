package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
)

type Title struct {
	ID          int64
	ISBN        string
	Title       string
	Subtitle    string
	Publisher   string
	Year        int
	Description string
	CoverURL    string
	Authors     []string
	Subjects    []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TitleInput is what a librarian fills in. The ISBN must already be
// normalised (or empty for books without one).
type TitleInput struct {
	ISBN        string
	Title       string
	Subtitle    string
	Publisher   string
	Year        int
	Description string
	CoverURL    string
	Authors     []string
	Subjects    []string
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func nullInt(n int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(n), Valid: n != 0}
}

func (s *Store) CreateTitle(ctx context.Context, in TitleInput, now time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `INSERT INTO titles
		(isbn, title, sort_title, subtitle, publisher, year, description, cover_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullString(in.ISBN), in.Title, domain.SortTitle(in.Title), in.Subtitle, in.Publisher, nullInt(in.Year),
		in.Description, in.CoverURL, fmtTime(now), fmtTime(now))
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrConflict
		}
		return 0, fmt.Errorf("insert title: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := setTitleAuthors(ctx, tx, id, in.Authors); err != nil {
		return 0, err
	}
	if err := setTitleSubjects(ctx, tx, id, in.Subjects); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (s *Store) UpdateTitle(ctx context.Context, id int64, in TitleInput, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `UPDATE titles SET
		isbn = ?, title = ?, sort_title = ?, subtitle = ?, publisher = ?, year = ?, description = ?,
		cover_url = ?, updated_at = ?
		WHERE id = ?`,
		nullString(in.ISBN), in.Title, domain.SortTitle(in.Title), in.Subtitle, in.Publisher, nullInt(in.Year),
		in.Description, in.CoverURL, fmtTime(now), id)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("update title: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if err := setTitleAuthors(ctx, tx, id, in.Authors); err != nil {
		return err
	}
	if err := setTitleSubjects(ctx, tx, id, in.Subjects); err != nil {
		return err
	}
	return tx.Commit()
}

func setTitleAuthors(ctx context.Context, tx *sql.Tx, titleID int64, names []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM title_authors WHERE title_id = ?`, titleID); err != nil {
		return err
	}
	for i, name := range names {
		var authorID int64
		err := tx.QueryRowContext(ctx,
			`INSERT INTO authors (name) VALUES (?)
			 ON CONFLICT (name) DO UPDATE SET name = authors.name
			 RETURNING id`, name).Scan(&authorID)
		if err != nil {
			return fmt.Errorf("author %q: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO title_authors (title_id, author_id, position) VALUES (?, ?, ?)`,
			titleID, authorID, i); err != nil {
			return err
		}
	}
	return nil
}

func setTitleSubjects(ctx context.Context, tx *sql.Tx, titleID int64, names []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM title_subjects WHERE title_id = ?`, titleID); err != nil {
		return err
	}
	for _, name := range names {
		var subjectID int64
		err := tx.QueryRowContext(ctx,
			`INSERT INTO subjects (name) VALUES (?)
			 ON CONFLICT (name) DO UPDATE SET name = subjects.name
			 RETURNING id`, name).Scan(&subjectID)
		if err != nil {
			return fmt.Errorf("subject %q: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO title_subjects (title_id, subject_id) VALUES (?, ?)`,
			titleID, subjectID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) TitleByID(ctx context.Context, id int64) (Title, error) {
	var t Title
	var isbn sql.NullString
	var year sql.NullInt64
	var created, updated string
	err := s.db.QueryRowContext(ctx, `SELECT id, isbn, title, subtitle, publisher, year,
		description, cover_url, created_at, updated_at FROM titles WHERE id = ?`, id).
		Scan(&t.ID, &isbn, &t.Title, &t.Subtitle, &t.Publisher, &year,
			&t.Description, &t.CoverURL, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Title{}, ErrNotFound
	}
	if err != nil {
		return Title{}, err
	}
	t.ISBN = isbn.String
	t.Year = int(year.Int64)
	t.CreatedAt, t.UpdatedAt = parseTime(created), parseTime(updated)

	if t.Authors, err = s.strings(ctx, `SELECT a.name FROM title_authors ta
		JOIN authors a ON a.id = ta.author_id WHERE ta.title_id = ? ORDER BY ta.position`, id); err != nil {
		return Title{}, err
	}
	if t.Subjects, err = s.strings(ctx, `SELECT s.name FROM title_subjects ts
		JOIN subjects s ON s.id = ts.subject_id WHERE ts.title_id = ? ORDER BY s.name`, id); err != nil {
		return Title{}, err
	}
	return t, nil
}

func (s *Store) strings(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// TitleSummary is one row of the catalogue list.
type TitleSummary struct {
	ID       int64
	Title    string
	Subtitle string
	Authors  string
	Year     int
	CoverURL string
	// Copies counts the copies that circulate (not lost or withdrawn);
	// Available only the ones on the shelf.
	Copies    int
	Available int
}

type TitleFilter struct {
	Query string
	// Subject and Author narrow the list to one subject or author by
	// exact (case-insensitive) name.
	Subject string
	Author  string
	// AvailableOnly keeps titles with a copy on the shelf.
	AvailableOnly bool
	Limit         int
	Offset        int
}

// ListTitles returns titles matching the filter, sorted by title, and how
// many match in total.
func (s *Store) ListTitles(ctx context.Context, f TitleFilter) ([]TitleSummary, int, error) {
	if f.Limit <= 0 {
		f.Limit = 25
	}
	var conds []string
	var args []any
	if q := strings.TrimSpace(f.Query); q != "" {
		like := likeContains(q)
		conds = append(conds, `(t.title LIKE ? ESCAPE '\' OR t.subtitle LIKE ? ESCAPE '\' OR t.isbn LIKE ? ESCAPE '\'
			OR EXISTS (SELECT 1 FROM title_authors ta JOIN authors a ON a.id = ta.author_id
			           WHERE ta.title_id = t.id AND a.name LIKE ? ESCAPE '\'))`)
		args = append(args, like, like, like, like)
	}
	if f.Subject != "" {
		conds = append(conds, `EXISTS (SELECT 1 FROM title_subjects ts JOIN subjects s ON s.id = ts.subject_id
			WHERE ts.title_id = t.id AND s.name = ?)`)
		args = append(args, f.Subject)
	}
	if f.Author != "" {
		conds = append(conds, `EXISTS (SELECT 1 FROM title_authors ta JOIN authors a ON a.id = ta.author_id
			WHERE ta.title_id = t.id AND a.name = ?)`)
		args = append(args, f.Author)
	}
	if f.AvailableOnly {
		conds = append(conds, `EXISTS (SELECT 1 FROM copies c WHERE c.title_id = t.id AND c.status = 'available')`)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM titles t `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.title, t.subtitle, t.year, t.cover_url,
		coalesce((SELECT group_concat(name, ', ') FROM (
			SELECT a.name FROM title_authors ta JOIN authors a ON a.id = ta.author_id
			WHERE ta.title_id = t.id ORDER BY ta.position)), ''),
		(SELECT count(*) FROM copies c WHERE c.title_id = t.id AND c.status NOT IN ('lost', 'withdrawn')),
		(SELECT count(*) FROM copies c WHERE c.title_id = t.id AND c.status = 'available')
		FROM titles t `+where+`
		ORDER BY t.sort_title, t.id
		LIMIT ? OFFSET ?`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []TitleSummary
	for rows.Next() {
		var t TitleSummary
		var year sql.NullInt64
		if err := rows.Scan(&t.ID, &t.Title, &t.Subtitle, &year, &t.CoverURL, &t.Authors, &t.Copies, &t.Available); err != nil {
			return nil, 0, err
		}
		t.Year = int(year.Int64)
		out = append(out, t)
	}
	return out, total, rows.Err()
}

type SubjectCount struct {
	Name   string
	Titles int
}

// Subjects lists every subject in use with how many titles have it.
func (s *Store) Subjects(ctx context.Context) ([]SubjectCount, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT s.name, count(*) FROM subjects s
		JOIN title_subjects ts ON ts.subject_id = s.id
		GROUP BY s.id ORDER BY s.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SubjectCount
	for rows.Next() {
		var sc SubjectCount
		if err := rows.Scan(&sc.Name, &sc.Titles); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

var ErrTitleInUse = errors.New("the title still has copies")

// DeleteTitle removes a title that was added by mistake. Titles with
// copies can't be deleted: withdraw the copies instead, so loan history
// keeps pointing somewhere.
func (s *Store) DeleteTitle(ctx context.Context, id int64) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var copies int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM copies WHERE title_id = ?`, id).Scan(&copies); err != nil {
			return err
		}
		if copies > 0 {
			return ErrTitleInUse
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM titles WHERE id = ?`, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		// Forget authors and subjects nothing refers to any more.
		if _, err := tx.ExecContext(ctx, `DELETE FROM authors WHERE id NOT IN (SELECT author_id FROM title_authors)`); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM subjects WHERE id NOT IN (SELECT subject_id FROM title_subjects)`)
		return err
	})
}

// ExportRow is a title with its copies' barcodes, in the shape the CSV
// import reads.
type ExportRow struct {
	Title    Title
	Barcodes []string
}

// ExportCatalogue returns every title with the barcodes of its copies that
// are still in the collection (not withdrawn), sorted by title.
func (s *Store) ExportCatalogue(ctx context.Context) ([]ExportRow, error) {
	ids, err := s.ids(ctx, `SELECT id FROM titles ORDER BY sort_title, id`)
	if err != nil {
		return nil, err
	}
	out := make([]ExportRow, 0, len(ids))
	for _, id := range ids {
		t, err := s.TitleByID(ctx, id)
		if err != nil {
			return nil, err
		}
		barcodes, err := s.strings(ctx, `SELECT barcode FROM copies WHERE title_id = ? AND status != 'withdrawn' ORDER BY barcode`, id)
		if err != nil {
			return nil, err
		}
		out = append(out, ExportRow{Title: t, Barcodes: barcodes})
	}
	return out, nil
}

// NewArrivals returns the titles added most recently that have a copy.
func (s *Store) NewArrivals(ctx context.Context, limit int) ([]TitleSummary, error) {
	ids, err := s.ids(ctx, `SELECT t.id FROM titles t
		WHERE EXISTS (SELECT 1 FROM copies c WHERE c.title_id = t.id AND c.status NOT IN ('lost', 'withdrawn'))
		ORDER BY t.created_at DESC, t.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	out := make([]TitleSummary, 0, len(ids))
	for _, id := range ids {
		t, err := s.TitleByID(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, TitleSummary{ID: t.ID, Title: t.Title, Subtitle: t.Subtitle,
			Authors: strings.Join(t.Authors, ", "), Year: t.Year, CoverURL: t.CoverURL})
	}
	return out, nil
}
