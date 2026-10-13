package domain

import "testing"

func TestLateFine(t *testing.T) {
	p := DefaultPolicy() // 25 cents a day, capped at 5.00
	due := mustDate(t, "2026-03-10")

	tests := []struct {
		name     string
		returned string
		want     int
	}{
		{"early", "2026-03-01", 0},
		{"on the due date", "2026-03-10", 0},
		{"one day late", "2026-03-11", 25},
		{"ten days late", "2026-03-20", 250},
		{"exactly at the cap", "2026-03-30", 500},
		{"one day past the cap", "2026-03-31", 500},
		{"months late hits the cap", "2026-06-10", 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.LateFine(due, mustDate(t, tt.returned)); got != tt.want {
				t.Errorf("LateFine = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestLateFineGraceDays(t *testing.T) {
	p := DefaultPolicy()
	p.FineGraceDays = 2
	due := mustDate(t, "2026-03-10")
	tests := []struct {
		returned string
		want     int
	}{
		{"2026-03-11", 0},
		{"2026-03-12", 0},
		// Past the grace days every late day is charged.
		{"2026-03-13", 75},
	}
	for _, tt := range tests {
		if got := p.LateFine(due, mustDate(t, tt.returned)); got != tt.want {
			t.Errorf("returned %s: LateFine = %d, want %d", tt.returned, got, tt.want)
		}
	}
}

func TestLateFineNoCap(t *testing.T) {
	p := DefaultPolicy()
	p.FineCap = 0
	due := mustDate(t, "2026-03-10")
	if got := p.LateFine(due, due.AddDays(100)); got != 2500 {
		t.Errorf("LateFine = %d, want 2500", got)
	}
}

func TestFormatMoney(t *testing.T) {
	p := DefaultPolicy()
	tests := []struct {
		cents int
		want  string
	}{
		{0, "€0.00"},
		{5, "€0.05"},
		{125, "€1.25"},
		{100000, "€1000.00"},
		{-50, "-€0.50"},
	}
	for _, tt := range tests {
		if got := p.FormatMoney(tt.cents); got != tt.want {
			t.Errorf("FormatMoney(%d) = %q, want %q", tt.cents, got, tt.want)
		}
	}

	for currency, want := range map[string]string{
		"lei": "12.50 lei",
		"kr":  "12.50 kr",
		"$":   "$12.50",
		"R$":  "R$12.50",
		"":    "12.50",
	} {
		if got := (Policy{Currency: currency}).FormatMoney(1250); got != want {
			t.Errorf("FormatMoney with %q = %q, want %q", currency, got, want)
		}
	}
}

func TestParseMoney(t *testing.T) {
	tests := []struct {
		in   string
		want int
		ok   bool
	}{
		{"2", 200, true},
		{"2.5", 250, true},
		{"2,50", 250, true},
		{" 0.05 ", 5, true},
		{"", 0, false},
		{"1.234", 0, false},
		{"abc", 0, false},
		{"-3", 0, false},
	}
	for _, tt := range tests {
		got, err := ParseMoney(tt.in)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("ParseMoney(%q) = %d, %v", tt.in, got, err)
		}
	}
}
