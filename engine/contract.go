package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// The engine contract (SAD §5.1), S2 subset. It grows additively: each slice
// adds only the members it uses. Struct field order is the JSON member order
// of the encoded output.

// OverdueGraceDays is the minimum time between the due date and the overdue
// date: overdue = due + max(buffer, OverdueGraceDays days) (EQ1). It is a
// display constant of the contract, not a rule field.
const OverdueGraceDays = 7

// Display thresholds of the bucket assignment (SAD §5.3 step 9; EQ27). They
// are always added to today, never subtracted from a due date.
const (
	dueSoonDays    = 30 // 🟠 when the due date is within today + 30 days
	upcomingMonths = 6  // 🟡 up to today + 6 months; beyond: ✅ ON_TRACK or notYetScheduled
)

// Input is everything the engine needs about one profile. Today is the
// calendar date in Asia/Ho_Chi_Minh, computed by the caller.
type Input struct {
	Today   Date         `json:"today"`
	Profile Profile      `json:"profile"`
	Doses   []DoseRecord `json:"doses"`
}

// Profile holds the person's data the engine uses.
type Profile struct {
	Birth RecalledDate `json:"birth"`
}

// DoseRecord is one dose the person received. One record may protect against
// several diseases (a combination vaccine).
type DoseRecord struct {
	ID       string       `json:"id"`
	Diseases []string     `json:"diseases"`
	Date     RecalledDate `json:"date"`
	// DoseNumbers maps a disease of this record to the dose number the user
	// entered for it (F04.1), from 1. A key that is not one of Diseases is
	// UNKNOWN_REFERENCE; a number outside 1..999 is OUT_OF_RANGE. Added in
	// S2.
	DoseNumbers map[string]DoseNumber `json:"doseNumbers,omitzero"`
}

// maxDoseNumber is the largest dose number Validate accepts.
const maxDoseNumber = 999

// DoseNumber is a user-entered dose number. It decodes any JSON number with
// an integral value (1, 1.0, 1e0), because JSON Schema's "integer" accepts
// all of them and the decoder must accept every schema-valid document (I1).
// Integrality is decided on the decimal text, as the schema does, so a
// fraction is a decode error even when it rounds to an integer in float64
// (1.0000000000000001), and so are a value beyond ±2^53 and any other JSON
// kind (I2). Validate checks the range 1..999.
type DoseNumber int

// UnmarshalJSON implements json.Unmarshaler. JSON null leaves n unchanged:
// contractjson's shape pass rejects it with the contract's null error.
func (n *DoseNumber) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	if len(b) == 0 || (b[0] != '-' && (b[0] < '0' || b[0] > '9')) {
		return fmt.Errorf("dose number %s: want a JSON number", b)
	}
	v, ok := parseIntegral(string(b))
	if !ok {
		return fmt.Errorf("dose number %s: want an integer", b)
	}
	*n = DoseNumber(v)
	return nil
}

// maxExactInteger is 2^53, the largest magnitude parseIntegral accepts.
const maxExactInteger = 1 << 53

// parseIntegral returns the value of a JSON number token when that value is
// an integer of magnitude at most 2^53. It works on the decimal digits, so
// no rounding can make a fraction look integral.
func parseIntegral(s string) (int64, bool) {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	mantissa, exponent := s, ""
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mantissa, exponent = s[:i], s[i+1:]
	}
	digits, fraction := mantissa, 0
	if i := strings.IndexByte(mantissa, '.'); i >= 0 {
		digits, fraction = mantissa[:i]+mantissa[i+1:], len(mantissa)-i-1
	}
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return 0, false
	}
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return 0, true // zero, whatever the exponent
	}
	exp := 0
	if exponent != "" {
		e, err := strconv.Atoi(strings.TrimPrefix(exponent, "+"))
		// The value is digits × 10^(e − fraction + trailing zeros), with
		// fraction and trailing zeros each at most len(s). Below −len(s) it
		// is a fraction; above len(s) + 16 it has more than 16 digits. An
		// exponent Atoi cannot parse is beyond both. The bounds also keep
		// the arithmetic below from overflowing.
		if err != nil || e < -len(s) || e > len(s)+16 {
			return 0, false
		}
		exp = e
	}
	exp -= fraction
	for strings.HasSuffix(digits, "0") {
		digits, exp = digits[:len(digits)-1], exp+1
	}
	if exp < 0 || len(digits)+exp > 16 {
		return 0, false
	}
	v, err := strconv.ParseInt(digits+strings.Repeat("0", exp), 10, 64)
	if err != nil || v > maxExactInteger {
		return 0, false
	}
	if neg {
		v = -v
	}
	return v, true
}

// Output is the assessment of every disease of the rule set, in the rule
// set's disease order.
type Output struct {
	RuleSetVersion string       `json:"ruleSetVersion"`
	Today          Date         `json:"today"`
	Assessments    []Assessment `json:"assessments"`
}

// Assessment is the result for one disease: exactly one of Bucket or Block.
// NextDose is absent when the series is complete and whenever Bucket is
// BucketUnknown: a ⚪ assessment never carries dates (SAD §5.1).
//
// Bucket BucketUnknown always comes with a blocking reason, and a blocking
// reason makes Bucket BucketUnknown unless a block of a higher precedence
// layer wins over it (SAD §5.3 step 10, layers 1–5, none in S2); the
// reasons it masks then stay in Reasons. Informational reasons never make
// it ⚪ on their own.
type Assessment struct {
	Disease  string    `json:"disease"`
	Bucket   Bucket    `json:"bucket,omitempty"`
	Block    Block     `json:"block,omitempty"`
	NextDose *NextDose `json:"nextDose,omitzero"`
	// Estimated is true when the assessment rests on a birth date or a dose
	// date that is not day-precise (or no birth date), on a level-4 dose or
	// on an implied dose (SAD §5.3 step 1, EQ20d). It is set on ⚪
	// assessments too, and omitted when false. Added in S2.
	Estimated bool     `json:"estimated,omitzero"`
	Reasons   []Reason `json:"reasons,omitempty"`
}

// NextDose is the next dose to give: its position in the series and its
// earliest allowed, due and overdue dates.
type NextDose struct {
	Number   int  `json:"number"`
	Earliest Date `json:"earliest"`
	Due      Date `json:"due"`
	Overdue  Date `json:"overdue"`
}

// Bucket is the attention level of an assessment.
type Bucket string

// Buckets (SAD §5.3 step 9).
const (
	BucketOverdue  Bucket = "overdue"  // 🔴 "Quá hạn" ("Overdue")
	BucketDue      Bucket = "due"      // 🟠 "Đến hạn" ("Due")
	BucketUpcoming Bucket = "upcoming" // 🟡 "Sắp tới" ("Upcoming")
	BucketUnknown  Bucket = "unknown"  // ⚪ "Chưa rõ" ("Unknown")
	BucketComplete Bucket = "complete" // ✅ complete or on series
)

// Block is a collapsed group outside the buckets.
type Block string

// Blocks (S2 subset).
const (
	// BlockNotYetScheduled is "Chưa tới lịch tiêm" ("Not yet scheduled"): no dose
	// given and dose 1 due more than 6 months after today (EQ2, EQ18).
	BlockNotYetScheduled Block = "notYetScheduled"
)

// Reason explains an assessment. The wording is generated by the clients
// from Code and Params.
type Reason struct {
	Code   ReasonCode   `json:"code"`
	Params ReasonParams `json:"params,omitzero"`
}

// ReasonParams holds every reason parameter; each code uses exactly the
// members its doc lists (SAD §5.4). Members are typed: IDs and enums are
// strings, counts are integers, lists are arrays. They are in alphabetical
// JSON-name order, every field is omitzero, and there are no booleans
// (omitzero would drop false). It grows additively.
type ReasonParams struct {
	// Checks are the minimums a dose fails, in catalog order: DOSE_INVALID,
	// DOSE_POSSIBLY_INVALID.
	Checks []Check `json:"checks,omitzero"`
	// Dose is a dose record ID: INVALID_DATE, OUT_OF_RANGE, DATE_IN_FUTURE,
	// DATE_BEFORE_BIRTH, UNEXPECTED_VALUE, DOSE_INVALID,
	// DOSE_POSSIBLY_INVALID, INTERVAL_UNVERIFIABLE, NUMBERING_CONFLICT,
	// PREVIOUS_DOSE_DATE_UNKNOWN, FIRST_DOSE_DATE_ASSUMED.
	Dose string `json:"dose,omitzero"`
	// Doses are dose record IDs in time order, at least two:
	// OVERLAPPING_ESTIMATED_DATES.
	Doses []string `json:"doses,omitzero"`
	// Hint is what the user can enter to resolve the reason:
	// BIRTH_DATE_IMPRECISE.
	Hint Hint `json:"hint,omitzero"`
	// Number is a series position, ≥ 1: the position a dose was checked for
	// (DOSE_INVALID, DOSE_POSSIBLY_INVALID, INTERVAL_UNVERIFIABLE; for a dose
	// beyond the series length, the position it sits at, or the number of
	// the level-4 record whose number implies doses that cannot fit), the
	// position a floating level-4 dose fails after the last dated dose
	// (DOSE_INVALID), or the number the user entered (NUMBERING_CONFLICT).
	Number int `json:"number,omitzero"`
	// Numbers are the positions of implied doses, ascending:
	// IMPLIED_EARLIER_DOSES.
	Numbers []int `json:"numbers,omitzero"`
	// Series is a series ID: ALTERNATIVE_PLANS.
	Series string `json:"series,omitzero"`
}

// Check is a dose-validity check that a dose can fail.
type Check string

// Checks, in catalog order.
const (
	CheckMinAge               Check = "minAge"
	CheckMinInterval          Check = "minInterval"
	CheckMinIntervalFromFirst Check = "minIntervalFromFirst"
)

// Hint is the input that would resolve a BIRTH_DATE_IMPRECISE reason.
type Hint string

// Hints.
const (
	// HintBirthMonth: "Nhập tháng sinh để gợi ý chính xác hơn" ("Enter the
	// birth month for a more accurate recommendation"), for a year-precision
	// birth date.
	HintBirthMonth Hint = "birthMonth"
	// HintFullBirthDate: "Nhập ngày sinh đầy đủ" ("Enter the full date of
	// birth"), for a month-precision birth date.
	HintFullBirthDate Hint = "fullBirthDate"
)

// ReasonCode is a code from the closed reason catalog (SAD §5.4).
//
// A code is blocking or informational (blocks). A blocking reason makes the
// assessment ⚪ BucketUnknown with no next dose; an informational reason
// never does so on its own.
type ReasonCode string

// Reason codes (S2 subset). Blocking unless the doc says informational.
const (
	// ReasonInvalidDate: a dose record of the disease has an unparseable
	// date (A9). Params: dose.
	ReasonInvalidDate ReasonCode = "INVALID_DATE"
	// ReasonOutOfRange: a dose record of the disease has a date outside
	// [1900-01-01, 2199-12-31]. Params: dose.
	ReasonOutOfRange ReasonCode = "OUT_OF_RANGE"
	// ReasonDateInFuture: a dose record of the disease is dated after today.
	// Params: dose.
	ReasonDateInFuture ReasonCode = "DATE_IN_FUTURE"
	// ReasonDateBeforeBirth: a dose record of the disease is dated before the
	// birth date. Params: dose.
	ReasonDateBeforeBirth ReasonCode = "DATE_BEFORE_BIRTH"
	// ReasonUnexpectedValue: a dose record of the disease has precision
	// unknown and a value (record-scoped, like INVALID_DATE). Params: dose.
	ReasonUnexpectedValue ReasonCode = "UNEXPECTED_VALUE"
	// ReasonOverlappingEstimatedDates: the date ranges of two or more dose
	// records overlap and not all of them have a user-entered number, so
	// their order is unknown (SAD §5.3 step 2, F04.2). Params: doses.
	ReasonOverlappingEstimatedDates ReasonCode = "OVERLAPPING_ESTIMATED_DATES"
	// ReasonNumberingConflict: a user-entered dose number repeats an earlier
	// record's number or contradicts the time order (A4). Params: dose (the
	// later record), number (its user-entered number).
	ReasonNumberingConflict ReasonCode = "NUMBERING_CONFLICT"
	// ReasonImpliedEarlierDoses: informational. A user-entered number above
	// the records makes the missing earlier doses implied: counted, undated
	// (F04.1). Params: numbers.
	ReasonImpliedEarlierDoses ReasonCode = "IMPLIED_EARLIER_DOSES"
	// ReasonBirthDateImprecise: the two ends of the birth-date range give
	// different results (SAD §5.3 step 4, F05.5). Params: hint.
	ReasonBirthDateImprecise ReasonCode = "BIRTH_DATE_IMPRECISE"
	// ReasonDoseInvalid: a dose violates a minimum at every end of its date
	// range and is not counted. Params: checks (the failed minimums), dose,
	// number (the position it was checked for).
	ReasonDoseInvalid ReasonCode = "DOSE_INVALID"
	// ReasonDosePossiblyInvalid: a dose violates a minimum at one end of its
	// date range only; it is counted (F05.10). Params: checks, dose, number.
	ReasonDosePossiblyInvalid ReasonCode = "DOSE_POSSIBLY_INVALID"
	// ReasonIntervalUnverifiable: informational. The dose before this one in
	// time is undated (level 4 or implied), so the minimum interval to it is
	// waived and the dose is counted (EQ8, EQ20b). Params: dose, number.
	ReasonIntervalUnverifiable ReasonCode = "INTERVAL_UNVERIFIABLE"
	// ReasonComplete: informational. Every dose of the series is counted.
	ReasonComplete ReasonCode = "COMPLETE"
	// ReasonPreviousDoseDateUnknown: the next dose cannot be dated because
	// dose k−1 is undated, or an unnumbered level-4 dose may be the latest
	// dose (SAD §5.3 step 7). Params: dose (that undated record).
	ReasonPreviousDoseDateUnknown ReasonCode = "PREVIOUS_DOSE_DATE_UNKNOWN"
	// ReasonNoBirthDate: no birth date, and the next dose needs one: no dose
	// is counted yet, or its minimum age is not implied by the intervals
	// (EQ19).
	ReasonNoBirthDate ReasonCode = "NO_BIRTH_DATE"
	// ReasonFirstDoseDateAssumed: informational. Dose 1 is undated, so the
	// earliest dated dose stands in for it in a first-dose anchor or
	// minIntervalFromFirst (EQ20c). Params: dose (the stand-in record).
	ReasonFirstDoseDateAssumed ReasonCode = "FIRST_DOSE_DATE_ASSUMED"
	// ReasonAlternativePlans: informational. The next dose merges
	// alternative plans (A7). Params: series.
	ReasonAlternativePlans ReasonCode = "ALTERNATIVE_PLANS"
	// ReasonOnTrack: informational. At least one dose counted and the next
	// dose is more than 6 months away.
	ReasonOnTrack ReasonCode = "ON_TRACK"
)
