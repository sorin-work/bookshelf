package web

import (
	"strings"
	"sync"
	"time"
)

// loginLimiter slows down password guessing: after maxFailures failed
// logins for the same email within window, further attempts are refused
// until the window has passed. It lives in memory, which is fine for a
// single instance.
type loginLimiter struct {
	mu          sync.Mutex
	maxFailures int
	window      time.Duration
	failures    map[string][]time.Time
}

func newLoginLimiter(maxFailures int, window time.Duration) *loginLimiter {
	return &loginLimiter{maxFailures: maxFailures, window: window, failures: map[string][]time.Time{}}
}

func limiterKey(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// recent drops failures older than the window and returns the rest.
func (l *loginLimiter) recent(key string, now time.Time) []time.Time {
	kept := l.failures[key][:0]
	for _, t := range l.failures[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.failures, key)
		return nil
	}
	l.failures[key] = kept
	return kept
}

// Allowed reports whether a login for email may be tried now.
func (l *loginLimiter) Allowed(email string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(limiterKey(email), now)) < l.maxFailures
}

func (l *loginLimiter) Fail(email string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := limiterKey(email)
	l.failures[key] = append(l.recent(key, now), now)
}

func (l *loginLimiter) Reset(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, limiterKey(email))
}
