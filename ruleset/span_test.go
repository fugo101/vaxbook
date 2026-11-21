package ruleset

import (
	"testing"
)

func TestParseSpan(t *testing.T) {
	t.Parallel()
	valid := map[string]Span{
		"0d":   {0, Days},
		"6w":   {6, Weeks},
		"2m":   {2, Months},
		"9y":   {9, Years},
		"14d":  {14, Days},
		"999m": {999, Months},
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
	invalid := []string{"", "d", "6", "06w", "1000d", "-1d", "+1d", "1.5m", "1x", "1D", " 1d", "1d ", "6w2d", "1 d"}
	for _, in := range invalid {
		if got, err := ParseSpan(in); err == nil {
			t.Errorf("ParseSpan(%q) = %v, want error", in, got)
		}
	}
}

func TestSpanText(t *testing.T) {
	t.Parallel()
	var s Span
	if err := s.UnmarshalText([]byte("12w")); err != nil || s != (Span{12, Weeks}) {
		t.Fatalf("UnmarshalText = %v, %v", s, err)
	}
	b, err := s.MarshalText()
	if err != nil || string(b) != "12w" {
		t.Fatalf("MarshalText = %q, %v", b, err)
	}
	if _, err := (Span{}).MarshalText(); err == nil {
		t.Error("the zero Span marshalled without error")
	}
	if _, err := (Span{N: 1000, Unit: Days}).MarshalText(); err == nil {
		t.Error("an out-of-range Span marshalled without error")
	}
}
