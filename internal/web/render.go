package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

//go:embed templates
var templateFiles embed.FS

//go:embed static
var staticFiles embed.FS

// page is what every template gets. Handlers put their own values in Data.
type page struct {
	Title string
	User  *store.User
	CSRF  string
	Flash string
	// Path is the request path, for highlighting the current nav link.
	Path string
	Data any
}

// templateFuncs are the helpers available in every template. Times are
// shown in the library's time zone.
func templateFuncs(loc *time.Location) template.FuncMap {
	return template.FuncMap{
		// args builds a map so a partial can get more than one value:
		// {{template "loan_table" args "Page" $ "Loans" .Loans}}
		"args": func(kv ...any) (map[string]any, error) {
			if len(kv)%2 != 0 {
				return nil, fmt.Errorf("args needs key/value pairs")
			}
			m := make(map[string]any, len(kv)/2)
			for i := 0; i < len(kv); i += 2 {
				k, ok := kv[i].(string)
				if !ok {
					return nil, fmt.Errorf("args key %v is not a string", kv[i])
				}
				m[k] = kv[i+1]
			}
			return m, nil
		},
		"hasPrefix": strings.HasPrefix,
		"money": func(cents int, currency string) string {
			return domain.Policy{Currency: currency}.FormatMoney(cents)
		},
		"date": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.In(loc).Format("2 Jan 2006")
		},
		"datetime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.In(loc).Format("2 Jan 15:04")
		},
	}
}

// parsePages builds one template set per page so each page can define its
// own "content" block on top of the shared layout and partials.
func parsePages(loc *time.Location) (map[string]*template.Template, error) {
	names, err := fs.Glob(templateFiles, "templates/*.html")
	if err != nil {
		return nil, err
	}
	pages := map[string]*template.Template{}
	for _, name := range names {
		base := path.Base(name)
		if base == "layout.html" {
			continue
		}
		t, err := template.New(base).Funcs(templateFuncs(loc)).ParseFS(templateFiles,
			"templates/layout.html", "templates/partials/*.html", name)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", base, err)
		}
		pages[base] = t
	}
	return pages, nil
}

// render writes a full page. It renders into a buffer first so a template
// error turns into a clean 500 instead of half a page.
func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, name string, p page) {
	p.Flash = s.takeFlash(w, r)
	s.execute(w, r, status, name, "layout", p)
}

func (s *Server) execute(w http.ResponseWriter, r *http.Request, status int, name, block string, p page) {
	t, ok := s.pages[name]
	if !ok {
		s.serverError(w, r, fmt.Errorf("no template %q", name))
		return
	}
	p.User = currentUser(r)
	p.CSRF = csrfToken(r)
	p.Path = r.URL.Path
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, block, p); err != nil {
		s.serverError(w, r, fmt.Errorf("render %s/%s: %w", name, block, err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	http.Error(w, "Something went wrong on our side. Please try again.", http.StatusInternalServerError)
}

func (s *Server) forbidden(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusForbidden, "error.html", page{Title: "Not allowed", Data: errorPage{
		Heading: "That page is for librarians",
		Message: "Ask a librarian at the desk if you need help with this.",
	}})
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusNotFound, "error.html", page{Title: "Not found", Data: errorPage{
		Heading: "We couldn't find that page",
		Message: "It may have been removed, or the link is wrong.",
	}})
}

type errorPage struct {
	Heading string
	Message string
}

func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fileServer.ServeHTTP(w, r)
	})
}
