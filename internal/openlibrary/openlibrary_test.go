package openlibrary

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const duneJSON = `{"ISBN:9780441172719": {
	"title": "Dune",
	"authors": [{"name": "Frank Herbert"}],
	"publishers": [{"name": "Ace"}, {"name": "Someone else"}],
	"publish_date": "August 2, 2005",
	"subjects": [
		{"name": "Dune (Imaginary place)"}, {"name": "Fiction"}, {"name": "Fiction, science fiction, general"},
		{"name": "Science fiction"}, {"name": "fiction"}, {"name": "Ecology"}, {"name": "Deserts"}, {"name": "Messiahs"},
		{"name": "Too many"}
	],
	"cover": {"medium": "https://covers.openlibrary.org/b/id/1-M.jpg"}
}}`

func TestLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/books" || r.URL.Query().Get("bibkeys") != "ISBN:9780441172719" {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(duneJSON))
	}))
	defer srv.Close()
	c := New(srv.URL + "/")

	b, err := c.Lookup(context.Background(), "9780441172719")
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "Dune" || b.Publisher != "Ace" || b.Year != 2005 || len(b.Authors) != 1 {
		t.Errorf("got %+v", b)
	}
	want := []string{"Fiction", "Science fiction", "Ecology", "Deserts", "Messiahs"}
	if len(b.Subjects) != len(want) {
		t.Fatalf("subjects %v", b.Subjects)
	}
	for i := range want {
		if b.Subjects[i] != want[i] {
			t.Errorf("subjects %v, want %v", b.Subjects, want)
			break
		}
	}

	if _, err := c.Lookup(context.Background(), "9780306406157"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown isbn: %v", err)
	}
}

func TestLookupServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer srv.Close()
	if _, err := New(srv.URL).Lookup(context.Background(), "9780441172719"); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}
