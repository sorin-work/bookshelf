package domain

import (
	"fmt"
	"time"
)

// Date is a calendar day with no time of day or zone attached. Due dates,
// card expiry and hold pickup deadlines are all whole days at the library.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

const dateLayout = "2006-01-02"

// DateOf returns the calendar day t falls on in t's own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{y, m, d}
}

func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("parse date %q: %w", s, err)
	}
	return DateOf(t), nil
}

func (d Date) IsZero() bool { return d == Date{} }

// Time returns midnight UTC at the start of d.
func (d Date) Time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.Time().Format(dateLayout)
}

// Format formats d with a time.Format layout.
func (d Date) Format(layout string) string {
	if d.IsZero() {
		return ""
	}
	return d.Time().Format(layout)
}

func (d Date) AddDays(n int) Date { return DateOf(d.Time().AddDate(0, 0, n)) }

func (d Date) AddYears(n int) Date { return DateOf(d.Time().AddDate(n, 0, 0)) }

func (d Date) Weekday() time.Weekday { return d.Time().Weekday() }

func (d Date) Before(o Date) bool { return d.Time().Before(o.Time()) }

func (d Date) After(o Date) bool { return d.Time().After(o.Time()) }

// DaysUntil returns the number of days from d to o; negative if o is
// earlier.
func (d Date) DaysUntil(o Date) int {
	return int(o.Time().Sub(d.Time()).Hours() / 24)
}

// MarshalText and UnmarshalText store a Date as 2006-01-02, which is how
// it appears in the saved lending policy.
func (d Date) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

func (d *Date) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		*d = Date{}
		return nil
	}
	parsed, err := ParseDate(string(b))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
