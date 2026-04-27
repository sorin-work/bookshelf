// Package openlibrary looks up book details by ISBN on Open Library
// (https://openlibrary.org), so librarians don't have to type them in.
package openlibrary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrNotFound = errors.New("open library doesn't know this ISBN")

// Book is the part of an Open Library record the catalogue uses.
type Book struct {
	Title     string
	Subtitle  string
	Authors   []string
	Publisher string
	Year      int
	Subjects  []string
	CoverURL  string
}

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 5 * time.Second},
	}
}

type named struct {
	Name string `json:"name"`
}

type record struct {
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle"`
	Authors     []named `json:"authors"`
	Publishers  []named `json:"publishers"`
	PublishDate string  `json:"publish_date"`
	Subjects    []named `json:"subjects"`
	Cover       struct {
		Medium string `json:"medium"`
	} `json:"cover"`
}

// Lookup fetches the book with the given ISBN-13.
func (c *Client) Lookup(ctx context.Context, isbn string) (Book, error) {
	q := url.Values{}
	q.Set("bibkeys", "ISBN:"+isbn)
	q.Set("format", "json")
	q.Set("jscmd", "data")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/books?"+q.Encode(), nil)
	if err != nil {
		return Book{}, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return Book{}, fmt.Errorf("open library: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Book{}, fmt.Errorf("open library: %s", res.Status)
	}
	var body map[string]record
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return Book{}, fmt.Errorf("open library: %w", err)
	}
	rec, ok := body["ISBN:"+isbn]
	if !ok || rec.Title == "" {
		return Book{}, ErrNotFound
	}
	return toBook(rec), nil
}

var yearPattern = regexp.MustCompile(`\b(1[4-9]\d\d|20\d\d)\b`)

// maxSubjects keeps the subject list short. Open Library has dozens per
// book, many of them very specific.
const maxSubjects = 5

func toBook(rec record) Book {
	b := Book{Title: strings.TrimSpace(rec.Title), Subtitle: strings.TrimSpace(rec.Subtitle), CoverURL: rec.Cover.Medium}
	for _, a := range rec.Authors {
		if n := strings.TrimSpace(a.Name); n != "" {
			b.Authors = append(b.Authors, n)
		}
	}
	if len(rec.Publishers) > 0 {
		b.Publisher = strings.TrimSpace(rec.Publishers[0].Name)
	}
	if m := yearPattern.FindString(rec.PublishDate); m != "" {
		b.Year, _ = strconv.Atoi(m)
	}
	seen := map[string]bool{}
	for _, s := range rec.Subjects {
		n := strings.TrimSpace(s.Name)
		key := strings.ToLower(n)
		if n == "" || len(n) > 30 || strings.ContainsAny(n, ",()") || seen[key] {
			continue
		}
		seen[key] = true
		b.Subjects = append(b.Subjects, n)
		if len(b.Subjects) == maxSubjects {
			break
		}
	}
	return b
}
