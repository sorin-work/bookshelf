package domain

import "testing"

func TestCardNumber(t *testing.T) {
	tests := []struct {
		id   int64
		want string
	}{
		{1, "2000000014"},
		{42, "2000000428"},
		{12345678, "2123456788"},
	}
	for _, tt := range tests {
		got := CardNumber(tt.id)
		if got != tt.want {
			t.Errorf("CardNumber(%d) = %s, want %s", tt.id, got, tt.want)
		}
		if _, err := NormalizeCardNumber(got); err != nil {
			t.Errorf("generated number %s does not validate: %v", got, err)
		}
	}
}

func TestNormalizeCardNumber(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"2000000428", "2000000428", true},
		{"2000 0004 28", "2000000428", true},
		{" 2000-0004-28 ", "2000000428", true},
		{"2000000429", "", false}, // wrong check digit
		{"2000000248", "", false}, // swapped digits
		{"200000042", "", false},
		{"20000004X8", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, err := NormalizeCardNumber(tt.in)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("NormalizeCardNumber(%q) = %q, %v", tt.in, got, err)
		}
	}
}

func TestFormatCardNumber(t *testing.T) {
	if got := FormatCardNumber("2000000428"); got != "2000 0004 28" {
		t.Errorf("got %q", got)
	}
	if got := FormatCardNumber("short"); got != "short" {
		t.Errorf("got %q", got)
	}
}
