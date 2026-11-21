package engine

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/fugo101/vaxbook/ruleset"
)

func daysInMonth(y, m int) int {
	return time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// eachDay calls f for every day of the given years, oldest first.
func eachDay(t *testing.T, fromYear, toYear int, f func(Date, time.Time)) {
	t.Helper()
	for tm := time.Date(fromYear, 1, 1, 0, 0, 0, 0, time.UTC); tm.Year() <= toYear; tm = tm.AddDate(0, 0, 1) {
		dt, err := NewDate(tm.Year(), int(tm.Month()), tm.Day())
		if err != nil {
			t.Fatal(err)
		}
		f(dt, tm)
	}
}

// Month addition keeps the day of month and clamps to the end of the target
// month, never overflowing into the next month (SAD §5.1). Covered around a
// leap year, the 400-year leap year 2000 and the non-leap 2100.
func TestPropertyAddMonths(t *testing.T) {
	t.Parallel()
	for _, years := range [][2]int{{1999, 2001}, {2099, 2101}} {
		eachDay(t, years[0], years[1], func(dt Date, tm time.Time) {
			y, m, day := dt.YMD()
			prev := dt
			for n := -24; n <= 120; n++ {
				got := dt.AddMonths(n)
				gy, gm, gd := got.YMD()
				total := y*12 + (m - 1) + n
				wy, wm := total/12, total%12+1
				if gy != wy || gm != wm {
					t.Fatalf("%s + %dm = %s: month overflow, want %04d-%02d", dt, n, got, wy, wm)
				}
				if want := min(day, daysInMonth(wy, wm)); gd != want {
					t.Fatalf("%s + %dm = %s: day %d, want %d", dt, n, got, gd, want)
				}
				if n == 0 && got != dt {
					t.Fatalf("%s + 0m = %s", dt, got)
				}
				if n > -24 && got.Before(prev) {
					t.Fatalf("not monotone: %s + %dm = %s < %s", dt, n, got, prev)
				}
				prev = got
				if day <= daysInMonth(wy, wm) {
					want := tm.AddDate(0, n, 0).Format(time.DateOnly)
					if got.String() != want {
						t.Fatalf("%s + %dm = %s, time says %s", dt, n, got, want)
					}
				}
			}
		})
	}
}

// AddMonths is not associative: going through a short month can lose days.
// AddMonths(AddMonths(d, a), b) keeps min(day, last day of the intermediate
// month, last day of the target month) while AddMonths(d, a+b) keeps
// min(day, last day of the target month), so the chained result is never
// later and is earlier exactly when the intermediate month clamps below both.
// The engine therefore always adds a span to its anchor in one step.
func TestPropertyAddMonthsNotAssociative(t *testing.T) {
	t.Parallel()
	jan31 := d(t, "2026-01-31")
	if got := jan31.AddMonths(1).AddMonths(1).String(); got != "2026-03-28" {
		t.Fatalf("2026-01-31 +1m +1m = %s, want 2026-03-28", got)
	}
	if got := jan31.AddMonths(2).String(); got != "2026-03-31" {
		t.Fatalf("2026-01-31 +2m = %s, want 2026-03-31", got)
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for range 20000 {
		dt := randomDate(rng, 1900, 2199)
		a, b := rng.IntN(37)-12, rng.IntN(37)-12
		chained, direct := dt.AddMonths(a).AddMonths(b), dt.AddMonths(a+b)
		_, _, day := dt.YMD()
		iy, im, _ := dt.AddMonths(a).YMD()
		ty, tm, _ := direct.YMD()
		_, _, cd := chained.YMD()
		_, _, dd := direct.YMD()
		if want := min(day, daysInMonth(iy, im), daysInMonth(ty, tm)); cd != want {
			t.Fatalf("%s %+dm %+dm = %s, want day %d", dt, a, b, chained, want)
		}
		if want := min(day, daysInMonth(ty, tm)); dd != want {
			t.Fatalf("%s %+dm = %s, want day %d", dt, a+b, direct, want)
		}
		if chained.After(direct) {
			t.Fatalf("%s %+dm %+dm = %s is after %+dm = %s", dt, a, b, chained, a+b, direct)
		}
	}
}

func randomDate(rng *rand.Rand, fromYear, toYear int) Date {
	lo, _ := NewDate(fromYear, 1, 1)
	hi, _ := NewDate(toYear, 12, 31)
	return lo.AddDays(rng.IntN(int(hi.days-lo.days) + 1))
}

// The ordinal is contiguous and agrees with the time package.
func TestPropertyOrdinalMatchesTime(t *testing.T) {
	t.Parallel()
	var prev Date
	eachDay(t, 1900, 2199, func(dt Date, tm time.Time) {
		if dt.String() != tm.Format(time.DateOnly) {
			t.Fatalf("%v formats as %s, want %s", tm, dt, tm.Format(time.DateOnly))
		}
		if !prev.IsZero() && dt.days-prev.days != 1 {
			t.Fatalf("%s - %s != 1 day", dt, prev)
		}
		if p, err := ParseDate(dt.String()); err != nil || p != dt {
			t.Fatalf("ParseDate(%s) = %v, %v", dt, p, err)
		}
		if got := dt.AddDays(10); got.String() != tm.AddDate(0, 0, 10).Format(time.DateOnly) {
			t.Fatalf("%s + 10d = %s", dt, got)
		}
		prev = dt
	})
}

// A year is 12 months and a week is 7 days.
func TestPropertySpanUnits(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(3, 4))
	for range 5000 {
		dt := randomDate(rng, 1900, 2199)
		n := rng.IntN(ruleset.MaxSpanN + 1)
		if dt.AddSpan(ruleset.Span{N: n, Unit: ruleset.Years}) != dt.AddMonths(12*n) {
			t.Fatalf("%s + %dy != + %dm", dt, n, 12*n)
		}
		if dt.AddSpan(ruleset.Span{N: n, Unit: ruleset.Weeks}) != dt.AddDays(7*n) {
			t.Fatalf("%s + %dw != + %dd", dt, n, 7*n)
		}
	}
}
