package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sorin-work/bookshelf/internal/db"
	"github.com/sorin-work/bookshelf/internal/domain"
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
		{Title: "100% Wolf", Authors: []string{"Jayne Lyons"}},
		{Title: "Snake_case for Beginners"},
	} {
		if _, err := s.CreateTitle(ctx, in, now); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		q    string
		want []string
	}{
		{"", []string{"100% Wolf", "Matilda", "Snake_case for Beginners", "The BFG", "The Hobbit"}},
		{"%", []string{"100% Wolf"}},
		{"0% w", []string{"100% Wolf"}},
		{"_", []string{"Snake_case for Beginners"}},
		{`\`, nil},
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

func TestListTitlesCountsOnlyCirculatingCopies(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id, err := s.CreateTitle(ctx, TitleInput{Title: "Matilda"}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []string{"M-1", "M-2", "M-3"} {
		if _, err := s.CreateCopy(ctx, id, CopyInput{Barcode: b}, now); err != nil {
			t.Fatal(err)
		}
	}
	lost, _ := s.CopyByBarcode(ctx, "M-1")
	gone, _ := s.CopyByBarcode(ctx, "M-2")
	if err := s.SetCopyStatus(ctx, lost.ID, domain.CopyLost); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCopyStatus(ctx, gone.ID, domain.CopyWithdrawn); err != nil {
		t.Fatal(err)
	}
	got, _, err := s.ListTitles(ctx, TitleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Copies != 1 || got[0].Available != 1 {
		t.Errorf("copies %d, available %d; want 1 and 1", got[0].Copies, got[0].Available)
	}
}

func TestListTitlesFilters(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	dune, _ := s.CreateTitle(ctx, TitleInput{Title: "Dune", Authors: []string{"Frank Herbert"}, Subjects: []string{"Science fiction"}}, now)
	if _, err := s.CreateTitle(ctx, TitleInput{Title: "Dune Messiah", Authors: []string{"Frank Herbert"}, Subjects: []string{"Science fiction"}}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTitle(ctx, TitleInput{Title: "Matilda", Authors: []string{"Roald Dahl"}, Subjects: []string{"Children"}}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateCopy(ctx, dune, CopyInput{Barcode: "D-1"}, now); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		f    TitleFilter
		want int
	}{
		{"subject", TitleFilter{Subject: "science FICTION"}, 2},
		{"author", TitleFilter{Author: "Roald Dahl"}, 1},
		{"author and query", TitleFilter{Author: "Frank Herbert", Query: "messiah"}, 1},
		{"available only", TitleFilter{AvailableOnly: true}, 1},
		{"unknown subject", TitleFilter{Subject: "Cooking"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, total, err := s.ListTitles(ctx, tt.f)
			if err != nil {
				t.Fatal(err)
			}
			if total != tt.want {
				t.Errorf("total = %d, want %d", total, tt.want)
			}
		})
	}

	subjects, err := s.Subjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(subjects) != 2 || subjects[0].Name != "Children" || subjects[1].Titles != 2 {
		t.Errorf("subjects %+v", subjects)
	}
}

func TestDeleteTitle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	gone, _ := s.CreateTitle(ctx, TitleInput{Title: "Typo", Authors: []string{"Only Here"}, Subjects: []string{"Oddities"}}, now)
	kept, _ := s.CreateTitle(ctx, TitleInput{Title: "Kept", Authors: []string{"Stays"}}, now)
	if _, err := s.CreateCopy(ctx, kept, CopyInput{Barcode: "K-1"}, now); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTitle(ctx, kept); !errors.Is(err, ErrTitleInUse) {
		t.Errorf("title with copies: %v", err)
	}
	if err := s.DeleteTitle(ctx, gone); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTitle(ctx, gone); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete twice: %v", err)
	}
	subjects, _ := s.Subjects(ctx)
	if len(subjects) != 0 {
		t.Errorf("orphan subjects kept: %+v", subjects)
	}
	var authors int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM authors`).Scan(&authors); err != nil {
		t.Fatal(err)
	}
	if authors != 1 {
		t.Errorf("%d authors left, want 1", authors)
	}
}
