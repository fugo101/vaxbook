package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// The engine contract (SAD §5.1), S4c subset. It grows additively: each slice
// adds only the members it uses. Struct field order is the JSON member order
// of the encoded output.

// OverdueGraceDays is the minimum time between the due date and the overdue
// date: overdue = due + max(buffer, OverdueGraceDays days) (EQ1), before
// the maximum-age cap; a capped overdue date may equal the due date (F05.4b,
// DUE_CAPPED_BY_MAX_AGE, SERIES_MAY_NOT_COMPLETE). It is a display constant
// of the contract, not a rule field.
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
	ID       string   `json:"id"`
	Diseases []string `json:"diseases"`
	// Product is the catalog product given, a product ID of the rule set
	// (retired ones included). An ID the rule set does not have is
	// UNKNOWN_PRODUCT, record-scoped: the record's diseases become ⚪
	// (issue #14, Q35). For a disease the product does not list, the dose
	// has an unknown product (Q36). Added in S3a.
	Product *string `json:"product,omitzero"`
	// FreeProduct is the free-text name of an off-catalog product (F04.1).
	// The engine reads only whether it is present; it never matches the
	// text against catalog names. With Product it is MUTUALLY_EXCLUSIVE. A
	// record with neither has an unknown product. Added in S3a.
	//
	// Both are pointers so that a present empty string, which is REQUIRED,
	// is told apart from an absent member.
	FreeProduct *string      `json:"freeProduct,omitzero"`
	Date        RecalledDate `json:"date"`
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
// NextDose is absent when the series is complete, whenever Bucket is
// BucketUnknown (a ⚪ assessment never carries dates, SAD §5.1) and for
// BlockOutOfAge.
//
// Bucket BucketUnknown always comes with a blocking reason, and a blocking
// reason makes Bucket BucketUnknown unless a block of a higher precedence
// layer wins over it (SAD §5.3 step 10, layers 1–5; since S4b layer 5's
// BlockOutOfAge from a maximum age); the reasons it masks then stay in
// Reasons. Informational reasons never make
// it ⚪ on their own.
type Assessment struct {
	Disease  string    `json:"disease"`
	Bucket   Bucket    `json:"bucket,omitempty"`
	Block    Block     `json:"block,omitempty"`
	NextDose *NextDose `json:"nextDose,omitzero"`
	// SeriesUsed holds the candidate series after SAD §5.3 step 3, in
	// rule-set order, complete candidates included (issue #14, Q26). It is
	// omitted after a step-1 or step-2 ⚪ and for ⚪ NO_BIRTH_DATE with no
	// dose given, where step 3 does not run, and for a BlockOutOfAge with no
	// dose given (issue #28, Q74). With no dose given and different
	// candidates across the birth-date range, it is the later end's list,
	// in a ⚪ too (issue #23, Q40), and omitted when the later end has no
	// candidate left (issue #28 In scope 6). It is also omitted for ⚪
	// NO_SERIES_IN_EFFECT, where no series is in effect today (issue #29).
	// Added in S3a.
	SeriesUsed []string `json:"seriesUsed,omitzero"`
	// Products are the non-retired catalog products for the next dose, in
	// snapshot order: the product of each candidate product series at the
	// next dose number, and for a generic candidate every non-retired
	// product of the disease with no series of its own. Present only with
	// NextDose and when not empty. Added in S3a.
	Products []string `json:"products,omitzero"`
	// Estimated is true when the assessment rests on a birth date or a dose
	// date that is not day-precise (or no birth date), on a level-4 dose or
	// on an implied dose (SAD §5.3 step 1, EQ20d), or when PRODUCT_NOT_IN_CATALOG
	// is given (EQ14). It is set on ⚪
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

// Blocks (S4c subset).
const (
	// BlockNotYetScheduled is "Chưa tới lịch tiêm" ("Not yet scheduled"): no dose
	// given and dose 1 due more than 6 months after today (EQ2, EQ18).
	BlockNotYetScheduled Block = "notYetScheduled"
	// BlockOutOfAge is "Không còn trong độ tuổi tiêm" ("No longer within the
	// vaccination age", F05.4a): no dose given and the person is past every
	// start-age bracket of the disease at the evaluated birth end (issue #23,
	// Q24), or certainly past a maximum age: with no dose given every
	// candidate is past its start limit (A3), and with doses given every
	// candidate is incomplete and certainly past its last allowed day for
	// the next dose (issue #28, step 10 layer 5). It has the reason
	// OUT_OF_AGE and no NextDose or Products. With no dose given it has no
	// SeriesUsed; with doses given it keeps SeriesUsed and the reasons
	// emitted before step 4 (IMPLIED_EARLIER_DOSES, AGE_AT_START_UNCERTAIN,
	// SAFEST_SERIES), and may carry masked reasons, blocking ones included
	// (Q74).
	BlockOutOfAge Block = "outOfAge"
)

// Reason explains an assessment. The wording is generated by the clients
// from Code and Params.
type Reason struct {
	Code   ReasonCode   `json:"code"`
	Params ReasonParams `json:"params,omitzero"`
}

// ReasonParams holds every reason parameter; each code uses exactly the
// members its doc lists (SAD §5.4). Members are typed: IDs and enums are
// strings, counts are integers, dates are Dates, lists are arrays. They are in alphabetical
// JSON-name order, every field is omitzero, and there are no booleans
// (omitzero would drop false). It grows additively.
type ReasonParams struct {
	// Candidates are series in rule-set order, at least two: the candidate
	// series (SAFEST_SERIES) or the matching start-age brackets of one
	// product group (AGE_AT_START_UNCERTAIN). Added in S3a.
	Candidates []SeriesRef `json:"candidates,omitzero"`
	// Checks are the checks a dose fails or may fail, in catalog order:
	// DOSE_INVALID (minimums only), DOSE_AFTER_MAX_AGE (maximum ages only),
	// DOSE_POSSIBLY_INVALID (any).
	Checks []Check `json:"checks,omitzero"`
	// Dose is a dose record ID: INVALID_DATE, OUT_OF_RANGE, DATE_IN_FUTURE,
	// DATE_BEFORE_BIRTH, UNEXPECTED_VALUE, UNKNOWN_PRODUCT, DOSE_INVALID,
	// DOSE_AFTER_MAX_AGE, DOSE_POSSIBLY_INVALID, INTERVAL_UNVERIFIABLE,
	// NUMBERING_CONFLICT,
	// PREVIOUS_DOSE_DATE_UNKNOWN, FIRST_DOSE_DATE_ASSUMED.
	Dose string `json:"dose,omitzero"`
	// Doses are dose record IDs: OVERLAPPING_ESTIMATED_DATES in time order,
	// at least two; PRODUCT_NOT_IN_CATALOG by ID, at least one (an
	// off-catalog dose can be undated, so time order is not always
	// defined).
	Doses []string `json:"doses,omitzero"`
	// Hint is what the user can enter to resolve the reason:
	// BIRTH_DATE_IMPRECISE.
	Hint Hint `json:"hint,omitzero"`
	// LastDay is the last allowed day of the next dose:
	// DUE_CAPPED_BY_MAX_AGE (L_k, the last day that leaves room to complete
	// the series), SERIES_MAY_NOT_COMPLETE (Lim, the last day the dose can
	// still be given). Never the zero Date. Added in S4b.
	LastDay Date `json:"lastDay,omitzero"`
	// Number is a series position, ≥ 1: the position a dose was checked for
	// (DOSE_INVALID, DOSE_AFTER_MAX_AGE, DOSE_POSSIBLY_INVALID,
	// INTERVAL_UNVERIFIABLE; for a dose beyond the series length, the
	// position it sits at, or the number of the level-4 record whose number
	// implies doses that cannot fit), the position a floating level-4 dose
	// fails after the last dated dose (DOSE_INVALID, DOSE_AFTER_MAX_AGE), or
	// the number the user entered (NUMBERING_CONFLICT).
	Number int `json:"number,omitzero"`
	// Numbers are series positions, ascending and without repeats: the
	// implied doses (IMPLIED_EARLIER_DOSES), or since S4b the doses whose
	// exact date would show whether the next dose can still be given
	// (OUT_OF_AGE_UNCERTAIN).
	Numbers []int `json:"numbers,omitzero"`
	// Series is a series ID: ALTERNATIVE_PLANS.
	Series string `json:"series,omitzero"`
}

// SeriesRef names a candidate series and its product. Product is omitted
// for a generic series, so no member is ever empty. Added in S3a.
type SeriesRef struct {
	Product string `json:"product,omitzero"`
	Series  string `json:"series"`
}

// Check is a dose-validity check that a dose can fail.
type Check string

// Checks, in catalog order.
const (
	CheckMinAge               Check = "minAge"
	CheckMinInterval          Check = "minInterval"
	CheckMinIntervalFromFirst Check = "minIntervalFromFirst"
	// CheckMaxAgeStart is dose 1's start limit: the earlier of the series'
	// maxAgeStart and its product group's last start-age bracket max, each
	// when present (issue #15, Q55). Added in S4a.
	CheckMaxAgeStart Check = "maxAgeStart"
	// CheckMaxAgeComplete is the series' maxAgeComplete, for every dose of
	// the series. Added in S4a.
	CheckMaxAgeComplete Check = "maxAgeComplete"
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

// Reason codes (S4c subset). Blocking unless the doc says informational.
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
	// ReasonUnknownProduct: a dose record of the disease names a product
	// the rule set does not have (record-scoped, like INVALID_DATE; issue
	// #14, Q35). Params: dose.
	ReasonUnknownProduct ReasonCode = "UNKNOWN_PRODUCT"
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
	// ReasonSafestSeries: informational. Two or more candidate series
	// remain after step 3, and the result is the safest over them (SAD §5.3
	// steps 3 and 7, F05.3). Params: candidates (every candidate, complete
	// ones included).
	ReasonSafestSeries ReasonCode = "SAFEST_SERIES"
	// ReasonBirthDateImprecise: the two ends of the birth-date range give
	// different results (SAD §5.3 step 4, F05.5). Params: hint.
	ReasonBirthDateImprecise ReasonCode = "BIRTH_DATE_IMPRECISE"
	// ReasonDoseInvalid: a dose violates a minimum at every end of its date
	// range and is not counted. Params: checks (the failed minimums), dose,
	// number (the position it was checked for).
	ReasonDoseInvalid ReasonCode = "DOSE_INVALID"
	// ReasonDosePossiblyInvalid: a dose violates a minimum at one end of its
	// date range only, or (from S4a) may be on or after a maximum age; it is
	// counted (F05.10). Params: checks, dose, number. A maximum-age check
	// carried from an implied slot has the carrier's dose and number, as for
	// ReasonDoseAfterMaxAge.
	ReasonDosePossiblyInvalid ReasonCode = "DOSE_POSSIBLY_INVALID"
	// ReasonIntervalUnverifiable: informational. The dose before this one in
	// time is undated (level 4 or implied), so the minimum interval to it is
	// waived and the dose is counted (EQ8, EQ20b). Params: dose, number.
	ReasonIntervalUnverifiable ReasonCode = "INTERVAL_UNVERIFIABLE"
	// ReasonComplete: informational. Every dose of every candidate series is
	// counted.
	ReasonComplete ReasonCode = "COMPLETE"
	// ReasonProductNotInCatalog: informational. The disease is complete and
	// some of its dose records have an off-catalog product (freeProduct), so
	// completion rests on every candidate series (EQ14). The assessment is
	// estimated. Params: doses (those records, by ID).
	ReasonProductNotInCatalog ReasonCode = "PRODUCT_NOT_IN_CATALOG"
	// ReasonPreviousDoseDateUnknown: the next dose cannot be dated because
	// dose k−1 is undated, or an unnumbered level-4 dose may be the latest
	// dose (SAD §5.3 step 7). Params: dose (that undated record).
	ReasonPreviousDoseDateUnknown ReasonCode = "PREVIOUS_DOSE_DATE_UNKNOWN"
	// ReasonNoBirthDate: no birth date, and the next dose needs one: no dose
	// is counted yet, its minimum age is not implied by the intervals
	// (EQ19), or the series has a maximum age to complete (A6, issue #15
	// Q59; S4a).
	ReasonNoBirthDate ReasonCode = "NO_BIRTH_DATE"
	// ReasonFirstDoseDateAssumed: informational. Dose 1 is undated, so the
	// earliest dated dose stands in for it in a first-dose anchor or
	// minIntervalFromFirst (EQ20c). Params: dose (the stand-in record).
	ReasonFirstDoseDateAssumed ReasonCode = "FIRST_DOSE_DATE_ASSUMED"
	// ReasonAlternativePlans: informational. The next dose merges
	// alternative plans (A7) of a candidate series at the next dose
	// number, one reason per such series (issue #14, Q28). Params: series.
	ReasonAlternativePlans ReasonCode = "ALTERNATIVE_PLANS"
	// ReasonOnTrack: informational. At least one dose counted and the next
	// dose is more than 6 months away.
	ReasonOnTrack ReasonCode = "ON_TRACK"
	// ReasonAgeAtStartUncertain: informational. With doses given, more
	// than one start-age bracket of a product group may contain the age at
	// dose 1, and the matching series with the most doses is used (SAD
	// §5.3 step 3, F05.5; issue #23). One reason per such group. Params:
	// candidates (every matching bracket's series, in rule-set order).
	// Added in S3b.
	ReasonAgeAtStartUncertain ReasonCode = "AGE_AT_START_UNCERTAIN"
	// ReasonOutOfAge: informational. The reason of BlockOutOfAge: no dose
	// given, and the person is past every start-age bracket at the
	// evaluated birth end (issue #23, Q24), or certainly past a maximum age
	// (issue #28). A ⚪ BIRTH_DATE_IMPRECISE can list it from one end of the
	// birth-date range. Added in S3b.
	ReasonOutOfAge ReasonCode = "OUT_OF_AGE"
	// ReasonDoseAfterMaxAge: a dose is certainly on or after a maximum age
	// under every candidate series that checks it, so it is not counted
	// (SAD §5.3 step 5, F05.10, EQ7, EQ16): dose 1 on or after its start
	// limit (maxAgeStart), or a dose at a position up to the series length
	// on or after maxAgeComplete. Maximum ages are exclusive bounds. An
	// implied slot's failure is carried, like a chain failure, by the
	// record after its run: a dated dose (in the series, or beyond its
	// length and not checked itself), the last trailing level-4 slot, or
	// the lowest level-4 record beyond the series. The reason then has the
	// carrier's dose and number, so number can exceed the series length and
	// a maxAgeStart check can come with a number above 1. Params: checks
	// (the failed maximum ages only), dose, number (the position it was
	// checked for; for a carried failure, the carrier's position, or the
	// number of the level-4 record beyond the series that carries it).
	// Added in S4a.
	ReasonDoseAfterMaxAge ReasonCode = "DOSE_AFTER_MAX_AGE"
	// ReasonCheckProductName: the candidate series disagree on a maximum age
	// (one gives a dose DOSE_AFTER_MAX_AGE, another counts it or is complete
	// without checking it; or, since S4b, at the next dose: one is out of
	// age while another is complete or can still be given, A1), and some
	// dose of the disease has an unknown product: none, an implied dose, or
	// a catalog product that does not list the disease (F05.3, EQ16; issue
	// #15 Q52, Q53; issue #28 Q69). No params. Added in S4a.
	ReasonCheckProductName ReasonCode = "CHECK_PRODUCT_NAME"
	// ReasonMixedProducts: the candidate series disagree on a maximum age,
	// at a given dose or (since S4b) at the next dose, and every dose of the
	// disease has a named product: a catalog product that lists the disease
	// (retired allowed) or an off-catalog product (F05.3, EQ16; issue #15
	// Q54). No params. Added in S4a.
	ReasonMixedProducts ReasonCode = "MIXED_PRODUCTS"
	// ReasonDueCappedByMaxAge: informational, a bucket reason. The due or
	// overdue date of the next dose was lowered to L_k, the last day from
	// which the series can still be completed before maxAgeComplete (and,
	// for dose 1, before the start limit): "hạn chót" ("last day"). A capped
	// next dose is never 🔴 (F05.4b, EQ15; issue #28). Params: lastDay.
	// Added in S4b.
	ReasonDueCappedByMaxAge ReasonCode = "DUE_CAPPED_BY_MAX_AGE"
	// ReasonSeriesMayNotComplete: informational, a bucket reason. The next
	// dose can still be given until lastDay (Lim), but no longer leaves room
	// to complete the series before maxAgeComplete: due and overdue are
	// capped at Lim and the next dose is never 🔴 (F05.4b case 2; issue #28
	// Q66). Params: lastDay. Added in S4b.
	ReasonSeriesMayNotComplete ReasonCode = "SERIES_MAY_NOT_COMPLETE"
	// ReasonOutOfAgeUncertain: the next dose is past a maximum age when
	// counted from the latest possible date of an earlier dose (the later
	// end of an imprecise date, or the EQ20c stand-in for an undated or
	// implied dose 1), but not from the earliest possible dates, so the
	// person may still be in age: "có thể đã quá tuổi tiêm mũi tiếp theo —
	// nhập ngày tiêm chính xác của mũi N để biết" ("the next dose may
	// already be past the age limit — enter the exact date of dose N to
	// find out"). No next dose is shown, since one counted from the
	// earliest dates could be past the limit (P1; SAD §5.3 step 7, owner
	// 2026-11-24). Params: numbers (the positions whose exact date would
	// decide). Added in S4b.
	ReasonOutOfAgeUncertain ReasonCode = "OUT_OF_AGE_UNCERTAIN"
	// ReasonNoSeriesInEffect: blocking. No series of the disease is in
	// effect today (validity windows, or a disease with no series at all),
	// so there is no rule to judge the history against; the disease is
	// never dropped from the output (SAD §5.3 step 3, F05.1, EQ26, issue
	// #29 Q82). It is the only reason of its assessment, after a step-1 or
	// step-2 ⚪. No params. Added in S4c.
	ReasonNoSeriesInEffect ReasonCode = "NO_SERIES_IN_EFFECT"
	// ReasonCatchUpAskDoctor: blocking. The next dose's earliest date that
	// the result would show is at or after birth + catchUpAfterAge of some
	// candidate at k, so a doctor decides instead of a schedule (SAD §5.3
	// step 7 "Catch-up", F05.2, EQ26, issue #29 Q78, Q91, Q92). It replaces
	// any clean next dose; out of age wins over it, and the step-7 reasons
	// stay as masked reasons. No params. Added in S4c.
	ReasonCatchUpAskDoctor ReasonCode = "CATCH_UP_ASK_DOCTOR"
)
