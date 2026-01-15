package store

import (
	"context"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
)

// Stats are the headline numbers on the circulation dashboard.
type Stats struct {
	OnLoan         int
	Overdue        int
	DueToday       int
	HoldsWaiting   int
	HoldsReady     int
	CheckoutsToday int
	ReturnsToday   int
	Titles         int
	Copies         int
	Available      int
	Members        int
	FinesOwed      int
}

// DashboardStats counts what is happening now. dayStart and dayEnd bound
// "today" in the library's time zone.
func (s *Store) DashboardStats(ctx context.Context, today domain.Date, dayStart, dayEnd time.Time) (Stats, error) {
	var st Stats
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM loans WHERE returned_at IS NULL),
		(SELECT count(*) FROM loans WHERE returned_at IS NULL AND due_on < ?1),
		(SELECT count(*) FROM loans WHERE returned_at IS NULL AND due_on = ?1),
		(SELECT count(*) FROM holds WHERE status = 'waiting'),
		(SELECT count(*) FROM holds WHERE status = 'ready'),
		(SELECT count(*) FROM loans WHERE checked_out_at >= ?2 AND checked_out_at < ?3),
		(SELECT count(*) FROM loans WHERE returned_at >= ?2 AND returned_at < ?3),
		(SELECT count(*) FROM titles),
		(SELECT count(*) FROM copies WHERE status NOT IN ('lost', 'withdrawn')),
		(SELECT count(*) FROM copies WHERE status = 'available'),
		(SELECT count(*) FROM users WHERE role = 'member'),
		(SELECT coalesce(sum(amount_cents), 0) FROM fines WHERE status = 'open')`,
		today.String(), fmtTime(dayStart), fmtTime(dayEnd)).Scan(
		&st.OnLoan, &st.Overdue, &st.DueToday, &st.HoldsWaiting, &st.HoldsReady,
		&st.CheckoutsToday, &st.ReturnsToday, &st.Titles, &st.Copies, &st.Available,
		&st.Members, &st.FinesOwed)
	return st, err
}

type PopularTitle struct {
	TitleID   int64
	Title     string
	Checkouts int
}

// PopularTitles returns the titles checked out most since a time.
func (s *Store) PopularTitles(ctx context.Context, since time.Time, limit int) ([]PopularTitle, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.title, count(*) AS n
		FROM loans l JOIN copies c ON c.id = l.copy_id JOIN titles t ON t.id = c.title_id
		WHERE l.checked_out_at >= ?
		GROUP BY t.id ORDER BY n DESC, t.title COLLATE NOCASE LIMIT ?`, fmtTime(since), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PopularTitle
	for rows.Next() {
		var p PopularTitle
		if err := rows.Scan(&p.TitleID, &p.Title, &p.Checkouts); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Activity is one checkout or return for the dashboard feed.
type Activity struct {
	At         time.Time
	Kind       string // "checkout" or "return"
	TitleID    int64
	Title      string
	UserID     int64
	MemberName string
}

func (s *Store) RecentActivity(ctx context.Context, limit int) ([]Activity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT at, kind, title_id, title, user_id, name FROM (
		SELECT l.checked_out_at AS at, 'checkout' AS kind, t.id AS title_id, t.title, u.id AS user_id, u.name
		FROM loans l JOIN copies c ON c.id = l.copy_id JOIN titles t ON t.id = c.title_id JOIN users u ON u.id = l.user_id
		UNION ALL
		SELECT l.returned_at, 'return', t.id, t.title, u.id, u.name
		FROM loans l JOIN copies c ON c.id = l.copy_id JOIN titles t ON t.id = c.title_id JOIN users u ON u.id = l.user_id
		WHERE l.returned_at IS NOT NULL
	) ORDER BY at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Activity
	for rows.Next() {
		var a Activity
		var at string
		if err := rows.Scan(&at, &a.Kind, &a.TitleID, &a.Title, &a.UserID, &a.MemberName); err != nil {
			return nil, err
		}
		a.At = parseTime(at)
		out = append(out, a)
	}
	return out, rows.Err()
}
