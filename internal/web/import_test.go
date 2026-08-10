package web

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func TestParseImport(t *testing.T) {
	tests := []struct {
		name     string
		csv      string
		rows     int
		problems []string
	}{
		{"minimal", "title\nDune\nMatilda\n", 2, nil},
		{"all columns", "isbn,title,authors,subjects,year,barcodes\n" +
			"0-441-17271-7,Dune,Frank Herbert,Science fiction;Classics,1965,D-01;D-02\n", 1, nil},
		{"byte order mark from excel", "\ufefftitle,year\nDune,1965\n", 1, nil},
		{"no title column", "name\nDune\n", 0, []string{"including one called title"}},
		{"unknown column", "title,price\nDune,10\n", 0, []string{`Unknown column "price"`}},
		{"empty file", "", 0, []string{"empty"}},
		{"header only", "title\n", 0, []string{"no titles"}},
		{"line problems", "title,isbn,year,barcodes\n" +
			",,1990,\n" +
			"Dune,9780441172710,1965,\n" +
			"Matilda,,19x,M 01\n" +
			"Dune again,0441172717,,\n" +
			"Dune twice,9780441172719,,\n", 5, []string{
			"Line 2: The title is missing.",
			"Line 3: ISBN 9780441172710",
			"Line 4: \"19x\" isn't a year",
			"Line 4: Barcode \"M 01\"",
			"Line 6: ISBN 9780441172719 is also on line 5.",
		}},
		{"blank lines are ignored", "title,year\nDune,1965\n,\n\nMatilda,1988\n", 2, nil},
		{"duplicate barcode", "title,barcodes\nA,X-01\nB,x-01\n", 2, []string{"Line 3: Barcode X-01 is also on line 2."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, problems := parseImport(strings.NewReader(tt.csv), 2026)
			if len(rows) != tt.rows {
				t.Errorf("got %d rows, want %d", len(rows), tt.rows)
			}
			var all []string
			for _, p := range problems {
				all = append(all, "Line "+itoa(int64(p.Line))+": "+p.Message)
			}
			joined := strings.Join(all, "\n")
			if len(tt.problems) == 0 && len(problems) > 0 {
				t.Errorf("unexpected problems:\n%s", joined)
			}
			for _, want := range tt.problems {
				if !strings.Contains(joined, want) {
					t.Errorf("problems don't mention %q:\n%s", want, joined)
				}
			}
		})
	}
}

func (c *client) upload(path, field, filename, content string) response {
	c.env.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField(csrfField, c.csrf())
	fw, _ := mw.CreateFormFile(field, filename)
	_, _ = fw.Write([]byte(content))
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, c.env.srv.URL+path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return c.do(req)
}

func TestImportTitles(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	lib.addTitle("title", "Dune", "isbn", "9780441172719")

	r := lib.upload("/titles/import", "file", "books.csv",
		"title,isbn,authors,barcodes\nDune,0441172717,Frank Herbert,\nMatilda,,Roald Dahl,M-01;M-02\n")
	wantStatus(t, r, http.StatusOK)
	wantBody(t, r, "Added 1 title and 2 copies", "skipped: Dune")
	wantBody(t, lib.get("/catalogue?q=matilda"), "2 available")

	// A barcode that's already taken stops the whole import.
	r = lib.upload("/titles/import", "file", "more.csv", "title,barcodes\nThe BFG,B-01\nThe Twits,M-01\n")
	wantStatus(t, r, http.StatusUnprocessableEntity)
	wantBody(t, r, "Nothing was imported", "Barcode M-01 is already used")
	if strings.Contains(lib.get("/catalogue").body, "The BFG") {
		t.Error("partial import")
	}

	wantStatus(t, env.member("Mo", "mo@example.com").upload("/titles/import", "file", "x.csv", "title\nX\n"), http.StatusForbidden)
}

func TestExportRoundTrip(t *testing.T) {
	env := newTestEnv(t)
	lib := env.librarian()
	loc := lib.addTitle("title", "Dune", "isbn", "9780441172719", "authors", "Frank Herbert\nBrian Herbert",
		"subjects", "Science fiction, Classics", "year", "1965")
	lib.post(loc+"/copies", titleValues("barcode", "D-01"))
	lib.post(loc+"/copies", titleValues("barcode", "D-02"))

	r := lib.get("/titles/export.csv")
	wantStatus(t, r, http.StatusOK)
	wantBody(t, r, "isbn,title,subtitle,authors,subjects,publisher,year,description,cover_url,barcodes\n",
		"9780441172719,Dune,,Frank Herbert; Brian Herbert,Classics; Science fiction,,1965,,,D-01; D-02\n")

	rows, problems := parseImport(strings.NewReader(r.body), 2026)
	if len(problems) > 0 || len(rows) != 1 || len(rows[0].Barcodes) != 2 || len(rows[0].Title.Authors) != 2 {
		t.Errorf("export doesn't import back: %+v %+v", rows, problems)
	}
}
