package web

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

const maxImportBytes = 2 << 20

// importRow is one title from an import file, already checked.
type importRow struct {
	Line     int
	Title    store.TitleInput
	Barcodes []string
}

type importProblem struct {
	Line    int
	Message string
}

// importColumns are the columns an import file may have; only title is
// required. Lists inside a cell are separated by semicolons.
var importColumns = []string{"isbn", "title", "subtitle", "authors", "subjects", "publisher", "year", "description", "cover_url", "barcodes"}

// parseImport reads a CSV of titles. It returns every problem it finds so
// the librarian can fix the file in one go.
func parseImport(r io.Reader, currentYear int) ([]importRow, []importProblem) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, []importProblem{{Line: 1, Message: "The file is empty or isn't a CSV file."}}
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))] = i
	}
	if _, ok := col["title"]; !ok {
		return nil, []importProblem{{Line: 1, Message: "The first row needs column names, including one called title."}}
	}
	for name := range col {
		known := false
		for _, c := range importColumns {
			known = known || c == name
		}
		if !known {
			return nil, []importProblem{{Line: 1, Message: fmt.Sprintf("Unknown column %q. Use: %s.", name, strings.Join(importColumns, ", "))}}
		}
	}

	var rows []importRow
	var problems []importProblem
	seenISBN := map[string]int{}
	seenBarcode := map[string]int{}
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		line, _ := cr.FieldPos(0)
		if err != nil {
			problems = append(problems, importProblem{Line: line, Message: "Couldn't read this line: " + err.Error()})
			continue
		}
		get := func(name string) string {
			if i, ok := col[name]; ok && i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		bad := func(format string, args ...any) {
			problems = append(problems, importProblem{Line: line, Message: fmt.Sprintf(format, args...)})
		}

		row := importRow{Line: line, Title: store.TitleInput{
			Title:       get("title"),
			Subtitle:    get("subtitle"),
			Publisher:   get("publisher"),
			Description: get("description"),
			CoverURL:    get("cover_url"),
			Authors:     splitList(get("authors"), ";"),
			Subjects:    splitList(get("subjects"), ";"),
		}}
		if strings.Join(rec, "") == "" {
			continue
		}
		if row.Title.Title == "" {
			bad("The title is missing.")
		} else if utf8.RuneCountInString(row.Title.Title) > 300 {
			bad("The title is longer than 300 characters.")
		}
		if v := get("isbn"); v != "" {
			isbn, err := domain.NormalizeISBN(v)
			switch {
			case err != nil:
				bad("ISBN %s: %s.", v, err)
			case seenISBN[isbn] != 0:
				bad("ISBN %s is also on line %d.", v, seenISBN[isbn])
			default:
				seenISBN[isbn] = line
				row.Title.ISBN = isbn
			}
		}
		if v := get("year"); v != "" {
			y, err := strconv.Atoi(v)
			if err != nil || y < 1450 || y > currentYear+1 {
				bad("%q isn't a year between 1450 and %d.", v, currentYear+1)
			}
			row.Title.Year = y
		}
		if v := row.Title.CoverURL; v != "" && !strings.HasPrefix(v, "https://") && !strings.HasPrefix(v, "http://") {
			bad("The cover URL has to start with https://.")
		}
		for _, b := range splitList(get("barcodes"), ";") {
			barcode, err := domain.NormalizeBarcode(b)
			switch {
			case err != nil:
				bad("Barcode %q: %s.", b, err)
			case seenBarcode[barcode] != 0:
				bad("Barcode %s is also on line %d.", barcode, seenBarcode[barcode])
			default:
				seenBarcode[barcode] = line
				row.Barcodes = append(row.Barcodes, barcode)
			}
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 && len(problems) == 0 {
		problems = append(problems, importProblem{Line: 2, Message: "There are no titles in the file."})
	}
	return rows, problems
}

type importPage struct {
	Problems []importProblem
	Added    int
	Skipped  []string
	Copies   int
}

func (s *Server) importPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "import.html", page{Title: "Import titles", Data: importPage{}})
}

// importTitles adds the titles in an uploaded CSV. Nothing is imported if
// any line has a problem. Titles whose ISBN is already in the catalogue
// are skipped.
func (s *Server) importTitles(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)
	file, _, err := r.FormFile("file")
	if err != nil {
		s.render(w, r, http.StatusUnprocessableEntity, "import.html", page{Title: "Import titles", Data: importPage{
			Problems: []importProblem{{Message: "Choose a CSV file of at most 2 MB."}},
		}})
		return
	}
	defer file.Close()

	rows, problems := parseImport(file, s.now().Year())
	for _, row := range rows {
		for _, b := range row.Barcodes {
			_, err := s.store.CopyByBarcode(r.Context(), b)
			if err == nil {
				problems = append(problems, importProblem{Line: row.Line, Message: "Barcode " + b + " is already used by another copy."})
			} else if !errors.Is(err, store.ErrNotFound) {
				s.serverError(w, r, err)
				return
			}
		}
	}
	if len(problems) > 0 {
		s.render(w, r, http.StatusUnprocessableEntity, "import.html", page{Title: "Import titles", Data: importPage{Problems: problems}})
		return
	}

	var d importPage
	for _, row := range rows {
		id, err := s.store.CreateTitle(r.Context(), row.Title, s.now())
		if errors.Is(err, store.ErrConflict) {
			d.Skipped = append(d.Skipped, row.Title.Title)
			continue
		}
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		d.Added++
		for _, b := range row.Barcodes {
			if _, err := s.store.CreateCopy(r.Context(), id, store.CopyInput{Barcode: b}, s.now()); err != nil {
				s.serverError(w, r, err)
				return
			}
			d.Copies++
		}
	}
	s.log.Info("titles imported", "added", d.Added, "skipped", len(d.Skipped), "copies", d.Copies, "by", currentUser(r).ID)
	s.render(w, r, http.StatusOK, "import.html", page{Title: "Import titles", Data: d})
}

// exportTitles downloads the catalogue in the same format the import
// reads, as a backup or to move to another system.
func (s *Server) exportTitles(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.ExportCatalogue(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="catalogue-%s.csv"`, s.today()))
	cw := csv.NewWriter(w)
	_ = cw.Write(importColumns)
	for _, row := range rows {
		t := row.Title
		year := ""
		if t.Year != 0 {
			year = strconv.Itoa(t.Year)
		}
		_ = cw.Write([]string{
			t.ISBN, csvSafe(t.Title), csvSafe(t.Subtitle), csvSafe(strings.Join(t.Authors, "; ")),
			csvSafe(strings.Join(t.Subjects, "; ")), csvSafe(t.Publisher), year, csvSafe(t.Description),
			t.CoverURL, strings.Join(row.Barcodes, "; "),
		})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		s.log.Error("write catalogue csv", "err", err)
	}
}
