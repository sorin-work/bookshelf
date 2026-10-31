package domain

import "testing"

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
