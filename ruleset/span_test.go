package ruleset

import (
	"testing"
)

func TestParseSpan(t *testing.T) {
	t.Parallel()
	valid := map[string]Span{
		"P0D":   {0, Days},
		"P6W":   {6, Weeks},
		"P2M":   {2, Months},
		"P9Y":   {9, Years},
		"P14D":  {14, Days},
		"P999M": {999, Months},
		"P999Y": {999, Years},
	}
	for in, want := range valid {
		got, err := ParseSpan(in)
		if err != nil || got != want {
			t.Errorf("ParseSpan(%q) = %v, %v; want %v", in, got, err, want)
		}
		if got.String() != in {
			t.Errorf("Span(%q).String() = %q", in, got.String())
		}
	}
	invalid := []string{
		"", "P", "D", "PD", "P6", "6W", "6w", "P6w", "p6W", "P06W", "P1000D", "P-1D", "-P1D", "+P1D", "P+1D",
		"P1.5M", "P1X", "PT6M", "P1Y6M", "P14W6D", " P1D", "P1D ", "P 1D", "P1 D", "P６W",
	}
	for _, in := range invalid {
		if got, err := ParseSpan(in); err == nil {
			t.Errorf("ParseSpan(%q) = %v, want error", in, got)
		}
	}
}

func TestSpanText(t *testing.T) {
	t.Parallel()
	var s Span
	if err := s.UnmarshalText([]byte("P12W")); err != nil || s != (Span{12, Weeks}) {
		t.Fatalf("UnmarshalText = %v, %v", s, err)
	}
	b, err := s.MarshalText()
	if err != nil || string(b) != "P12W" {
		t.Fatalf("MarshalText = %q, %v", b, err)
	}
	if _, err := (Span{}).MarshalText(); err == nil {
		t.Error("the zero Span marshalled without error")
	}
	if _, err := (Span{N: 1000, Unit: Days}).MarshalText(); err == nil {
		t.Error("an out-of-range Span marshalled without error")
	}
}
