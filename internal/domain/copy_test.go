package domain

import "testing"

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

func TestNextBarcode(t *testing.T) {
	tests := map[string]string{
		"BK-0001": "BK-0002",
		"BK-0099": "BK-0100",
		"BK-9999": "BK-10000",
		"42":      "43",
		"LIB-A":   "",
		"":        "",
	}
	for in, want := range tests {
		if got := NextBarcode(in); got != want {
			t.Errorf("NextBarcode(%q) = %q, want %q", in, got, want)
		}
	}
}
