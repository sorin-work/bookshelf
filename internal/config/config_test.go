package config

import (
	"log/slog"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	c, err := load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":4409" || c.DBPath != "bookshelf.db" {
		t.Errorf("unexpected defaults: %+v", c)
	}
	if c.LogLevel != slog.LevelInfo || c.LogFormat != "json" {
		t.Errorf("unexpected log defaults: %v %q", c.LogLevel, c.LogFormat)
	}
	if c.SessionTTL != 14*24*time.Hour {
		t.Errorf("SessionTTL = %v", c.SessionTTL)
	}
	if c.SeedDemo || c.SecureCookies {
		t.Error("booleans should default to false")
	}
	if c.Location != time.UTC {
		t.Errorf("Location = %v", c.Location)
	}
	if !c.AllowSignup || c.LibraryName != "Bookshelf" {
		t.Errorf("AllowSignup %v, LibraryName %q", c.AllowSignup, c.LibraryName)
	}
	if c.OpenLibraryURL != "https://openlibrary.org" {
		t.Errorf("OpenLibraryURL = %q", c.OpenLibraryURL)
	}
}

func TestLoadOverrides(t *testing.T) {
	c, err := load(env(map[string]string{
		"ADDR":             ":8080",
		"DB_PATH":          "/data/library.db",
		"LOG_LEVEL":        "debug",
		"LOG_FORMAT":       "text",
		"SECURE_COOKIES":   "true",
		"SEED_DEMO":        "1",
		"SESSION_TTL":      "2h",
		"LIBRARY_TIMEZONE": "Europe/Bucharest",
		"ALLOW_SIGNUP":     "false",
		"LIBRARY_NAME":     "Mill Street Library",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" || c.DBPath != "/data/library.db" {
		t.Errorf("got %+v", c)
	}
	if c.LogLevel != slog.LevelDebug || c.LogFormat != "text" {
		t.Errorf("log settings: %v %q", c.LogLevel, c.LogFormat)
	}
	if !c.SecureCookies || !c.SeedDemo || c.SessionTTL != 2*time.Hour {
		t.Errorf("got %+v", c)
	}
	if c.AllowSignup || c.LibraryName != "Mill Street Library" {
		t.Errorf("AllowSignup %v, LibraryName %q", c.AllowSignup, c.LibraryName)
	}
	if c.Location.String() != "Europe/Bucharest" {
		t.Errorf("Location = %v", c.Location)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"bad level", map[string]string{"LOG_LEVEL": "loud"}},
		{"bad format", map[string]string{"LOG_FORMAT": "xml"}},
		{"bad bool", map[string]string{"SEED_DEMO": "yes please"}},
		{"bad ttl", map[string]string{"SESSION_TTL": "forever"}},
		{"short ttl", map[string]string{"SESSION_TTL": "10s"}},
		{"bad time zone", map[string]string{"LIBRARY_TIMEZONE": "Mars/Olympus"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := load(env(tt.env)); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestOpenLibraryOff(t *testing.T) {
	c, err := load(env(map[string]string{"OPENLIBRARY_URL": "off"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.OpenLibraryURL != "" {
		t.Errorf("OpenLibraryURL = %q", c.OpenLibraryURL)
	}
}
