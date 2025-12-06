package domain

import (
	"errors"
	"testing"
)

func TestNormalizeISBN(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		err  error
	}{
		{"isbn-13 plain", "9780306406157", "9780306406157", nil},
		{"isbn-13 hyphenated", "978-0-306-40615-7", "9780306406157", nil},
		{"isbn-13 with spaces", " 978 0 306 40615 7 ", "9780306406157", nil},
		{"isbn-13 979 prefix", "9791034304127", "9791034304127", nil},
		{"isbn-10 converted", "0306406152", "9780306406157", nil},
		{"isbn-10 hyphenated", "0-306-40615-2", "9780306406157", nil},
		{"isbn-10 with X", "080442957X", "9780804429573", nil},
		{"isbn-13 bad checksum", "9780306406158", "", ErrISBNChecksum},
		{"isbn-10 bad checksum", "0306406153", "", ErrISBNChecksum},
		{"isbn-13 bad prefix", "1230306406157", "", ErrISBNPrefix},
		{"too short", "12345", "", ErrISBNLength},
		{"too long", "97803064061570", "", ErrISBNLength},
		{"empty", "", "", ErrISBNLength},
		{"letters", "97803064O6157", "", ErrISBNChars},
		{"X not last", "08044X9575", "", ErrISBNChars},
		{"X in isbn-13", "978030640615X", "", ErrISBNChars},
		{"junk of odd length", "abc", "", ErrISBNChars},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeISBN(tt.in)
			if !errors.Is(err, tt.err) {
				t.Fatalf("NormalizeISBN(%q) error = %v, want %v", tt.in, err, tt.err)
			}
			if got != tt.want {
				t.Errorf("NormalizeISBN(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestISBN13To10(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"9780306406157", "0306406152"},
		{"9780804429573", "080442957X"},
		{"9791034304127", ""},
		{"123", ""},
	}
	for _, tt := range tests {
		if got := ISBN13To10(tt.in); got != tt.want {
			t.Errorf("ISBN13To10(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestISBNRoundTrip(t *testing.T) {
	for _, isbn10 := range []string{"0306406152", "080442957X", "0140449132", "0451524934"} {
		if got := ISBN13To10(ISBN10To13(isbn10)); got != isbn10 {
			t.Errorf("round trip of %s gave %s", isbn10, got)
		}
	}
}

func TestNormalizeBarcode(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"b00042", "B00042", true},
		{"  LIB-2024-001 \n", "LIB-2024-001", true},
		{"ab", "", false},
		{"has space", "", false},
		{"ünïcode", "", false},
		{"123456789012345678901234567890123", "", false},
	}
	for _, tt := range tests {
		got, err := NormalizeBarcode(tt.in)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("NormalizeBarcode(%q) = %q, %v", tt.in, got, err)
		}
	}
}
