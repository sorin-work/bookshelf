package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sorin-work/bookshelf/internal/db"
)

var now = time.Date(2026, time.January, 5, 18, 0, 0, 0, time.UTC)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := db.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	return New(conn)
}

func TestTitlesRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.CreateTitle(ctx, TitleInput{
		ISBN:     "9780441172719",
		Title:    "Dune",
		Year:     1965,
		Authors:  []string{"Frank Herbert"},
		Subjects: []string{"Science fiction", "Classics"},
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.TitleByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Dune" || got.Year != 1965 || got.ISBN != "9780441172719" {
		t.Errorf("got %+v", got)
	}
	if len(got.Authors) != 1 || got.Authors[0] != "Frank Herbert" {
		t.Errorf("authors = %v", got.Authors)
	}
	if len(got.Subjects) != 2 || got.Subjects[0] != "Classics" {
		t.Errorf("subjects = %v", got.Subjects)
	}

	// Same ISBN again is a conflict.
	if _, err := s.CreateTitle(ctx, TitleInput{ISBN: "9780441172719", Title: "Dune again"}, now); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate isbn: %v", err)
	}
	// Books without an ISBN don't clash with each other.
	for i := 0; i < 2; i++ {
		if _, err := s.CreateTitle(ctx, TitleInput{Title: "Parish newsletter"}, now); err != nil {
			t.Errorf("title without isbn: %v", err)
		}
	}

	err = s.UpdateTitle(ctx, id, TitleInput{
		ISBN: "9780441172719", Title: "Dune", Subtitle: "Deluxe edition",
		Authors: []string{"Frank Herbert", "Brian Herbert"},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = s.TitleByID(ctx, id)
	if got.Subtitle != "Deluxe edition" || len(got.Authors) != 2 || got.Authors[1] != "Brian Herbert" || len(got.Subjects) != 0 {
		t.Errorf("after update: %+v", got)
	}
	if err := s.UpdateTitle(ctx, 999, TitleInput{Title: "x"}, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing: %v", err)
	}
}

func TestListTitlesSearch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, in := range []TitleInput{
		{Title: "The Hobbit", Authors: []string{"J. R. R. Tolkien"}},
		{Title: "Matilda", Authors: []string{"Roald Dahl"}},
		{Title: "The BFG", Authors: []string{"Roald Dahl"}},
	} {
		if _, err := s.CreateTitle(ctx, in, now); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		q    string
		want []string
	}{
		{"", []string{"Matilda", "The BFG", "The Hobbit"}},
		{"dahl", []string{"Matilda", "The BFG"}},
		{"hobb", []string{"The Hobbit"}},
		{"nothing like it", nil},
	}
	for _, tt := range tests {
		got, total, err := s.ListTitles(ctx, TitleFilter{Query: tt.q})
		if err != nil {
			t.Fatal(err)
		}
		if total != len(tt.want) || len(got) != len(tt.want) {
			t.Errorf("%q: got %d rows (total %d), want %d", tt.q, len(got), total, len(tt.want))
			continue
		}
		for i := range got {
			if got[i].Title != tt.want[i] {
				t.Errorf("%q: row %d = %q, want %q", tt.q, i, got[i].Title, tt.want[i])
			}
		}
	}
	got, _, _ := s.ListTitles(ctx, TitleFilter{Query: "dahl"})
	if got[0].Authors != "Roald Dahl" {
		t.Errorf("authors column = %q", got[0].Authors)
	}
}
