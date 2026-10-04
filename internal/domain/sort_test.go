package domain

import "testing"

func TestSortTitle(t *testing.T) {
	tests := map[string]string{
		"The Hobbit":               "hobbit",
		"A Tale of Two Cities":     "tale of two cities",
		"An Inspector Calls":       "inspector calls",
		"Theatre of the Absurd":    "theatre of the absurd",
		"Anne of Green Gables":     "anne of green gables",
		"  the   BFG":              "bfg",
		"The":                      "the",
		"And Then There Were None": "and then there were none",
	}
	for in, want := range tests {
		if got := SortTitle(in); got != want {
			t.Errorf("SortTitle(%q) = %q, want %q", in, got, want)
		}
	}
}
