package web

import (
	"testing"
	"time"
)

func TestLoginLimiter(t *testing.T) {
	l := newLoginLimiter(3, 10*time.Minute)
	start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		if !l.Allowed("a@example.com", start) {
			t.Fatalf("blocked after %d failures", i)
		}
		l.Fail("a@example.com", start.Add(time.Duration(i)*time.Minute))
	}
	if l.Allowed(" A@Example.com ", start.Add(3*time.Minute)) {
		t.Error("allowed after three failures")
	}
	// The first failure drops out of the window at 10 minutes.
	if !l.Allowed("a@example.com", start.Add(10*time.Minute)) {
		t.Error("still blocked once the oldest failure expired")
	}
	l.Reset("a@example.com")
	if len(l.failures) != 0 {
		t.Errorf("failures kept after reset: %v", l.failures)
	}
}
