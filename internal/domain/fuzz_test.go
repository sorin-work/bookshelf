package domain

import "testing"

// Whatever NormalizeISBN accepts must be a valid ISBN-13 that normalises
// to itself.
func FuzzNormalizeISBN(f *testing.F) {
	for _, seed := range []string{"9780306406157", "0-306-40615-2", "080442957x", "ISBN 978 0 306", "", "XXXXXXXXXX"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		out, err := NormalizeISBN(in)
		if err != nil {
			return
		}
		if len(out) != 13 {
			t.Fatalf("NormalizeISBN(%q) = %q, not 13 digits", in, out)
		}
		again, err := NormalizeISBN(out)
		if err != nil || again != out {
			t.Fatalf("NormalizeISBN(%q) = %q, %v; not stable", out, again, err)
		}
	})
}

func FuzzNormalizeCardNumber(f *testing.F) {
	for _, seed := range []string{"2000000428", "2000 0004 28", "1234"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		out, err := NormalizeCardNumber(in)
		if err != nil {
			return
		}
		if FormatCardNumber(out) == out {
			t.Fatalf("%q was not formatted", out)
		}
		if again, err := NormalizeCardNumber(FormatCardNumber(out)); err != nil || again != out {
			t.Fatalf("formatted card %q does not read back", out)
		}
	})
}
