package domain

import (
	"testing"
	"time"
)

func TestDate(t *testing.T) {
	d, err := ParseDate("2026-02-27")
	if err != nil {
		t.Fatal(err)
	}
	if d.String() != "2026-02-27" || d.Weekday() != time.Friday {
		t.Errorf("got %s on %s", d, d.Weekday())
	}
	if got := d.AddDays(2); got.String() != "2026-03-01" {
		t.Errorf("AddDays crossed February wrong: %s", got)
	}
	if got := d.AddDays(-58); got.String() != "2025-12-31" {
		t.Errorf("AddDays backwards: %s", got)
	}
	leap, _ := ParseDate("2028-02-29")
	if got := leap.AddYears(1); got.String() != "2029-03-01" {
		t.Errorf("AddYears from a leap day: %s", got)
	}
	if !d.Before(d.AddDays(1)) || d.After(d.AddDays(1)) || d.Before(d) {
		t.Error("comparisons are off")
	}
	if n := d.DaysUntil(d.AddDays(30)); n != 30 {
		t.Errorf("DaysUntil = %d", n)
	}
	if n := d.AddDays(5).DaysUntil(d); n != -5 {
		t.Errorf("DaysUntil backwards = %d", n)
	}
	if _, err := ParseDate("27/02/2026"); err == nil {
		t.Error("expected an error")
	}
	if (Date{}).String() != "" || !(Date{}).IsZero() {
		t.Error("zero date")
	}
}

func TestDateOfUsesLocation(t *testing.T) {
	bucharest, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		t.Skip("no tz database")
	}
	late := time.Date(2026, 3, 10, 23, 30, 0, 0, time.UTC)
	if got := DateOf(late.In(bucharest)); got.String() != "2026-03-11" {
		t.Errorf("got %s", got)
	}
}

func TestDateText(t *testing.T) {
	d := Date{2026, time.March, 9}
	b, _ := d.MarshalText()
	if string(b) != "2026-03-09" {
		t.Errorf("MarshalText = %s", b)
	}
	var back Date
	if err := back.UnmarshalText(b); err != nil || back != d {
		t.Errorf("UnmarshalText = %v, %v", back, err)
	}
	if err := back.UnmarshalText([]byte("9 March")); err == nil {
		t.Error("expected an error")
	}
}
