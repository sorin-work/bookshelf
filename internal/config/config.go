// Package config reads settings from the environment.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr      string
	DBPath    string
	LogLevel  slog.Level
	LogFormat string
	// SecureCookies sets the Secure flag on session cookies. Turn it off
	// only when serving plain http on localhost.
	SecureCookies bool
	SessionTTL    time.Duration
	SeedDemo      bool
	// Location is the library's time zone. Due dates and "today" are
	// worked out in it.
	Location *time.Location
}

func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	c := Config{
		Addr:      envOr(getenv, "ADDR", ":4409"),
		DBPath:    envOr(getenv, "DB_PATH", "bookshelf.db"),
		LogFormat: envOr(getenv, "LOG_FORMAT", "json"),
	}

	if err := c.LogLevel.UnmarshalText([]byte(envOr(getenv, "LOG_LEVEL", "info"))); err != nil {
		return c, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	if c.LogFormat != "json" && c.LogFormat != "text" {
		return c, fmt.Errorf("LOG_FORMAT must be json or text, got %q", c.LogFormat)
	}

	var err error
	if c.SecureCookies, err = boolEnv(getenv, "SECURE_COOKIES", false); err != nil {
		return c, err
	}
	if c.SeedDemo, err = boolEnv(getenv, "SEED_DEMO", false); err != nil {
		return c, err
	}
	if c.SessionTTL, err = time.ParseDuration(envOr(getenv, "SESSION_TTL", "336h")); err != nil {
		return c, fmt.Errorf("SESSION_TTL: %w", err)
	}
	if c.SessionTTL < time.Minute {
		return c, fmt.Errorf("SESSION_TTL must be at least a minute")
	}
	if c.Location, err = time.LoadLocation(envOr(getenv, "LIBRARY_TIMEZONE", "UTC")); err != nil {
		return c, fmt.Errorf("LIBRARY_TIMEZONE: %w", err)
	}
	return c, nil
}

func envOr(getenv func(string) string, key, def string) string {
	if v := strings.TrimSpace(getenv(key)); v != "" {
		return v
	}
	return def
}

func boolEnv(getenv func(string) string, key string, def bool) (bool, error) {
	v := strings.TrimSpace(getenv(key))
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: %q is not a boolean", key, v)
	}
	return b, nil
}
