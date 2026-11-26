package ruleset

// Snapshot is an immutable rule set version: the S4c subset of SAD §5.2.
// Fields are added additively by later slices.
type Snapshot struct {
	Version string `json:"version"`
	// Illustrative marks a fixture or demo rule set, not medically verified;
	// the publish path never sets it.
	Illustrative bool      `json:"illustrative,omitzero"`
	Diseases     []Disease `json:"diseases"`
	// Products is the vaccine product catalog, in display order. Added in
	// S3a.
	Products []Product `json:"products,omitzero"`
	Series   []Series  `json:"series"`
}

// Disease is a disease the engine assesses, in display order.
type Disease struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Product is a vaccine product of the catalog (SAD §5.2, F11.3). Added in
// S3a.
type Product struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Diseases are the diseases the product protects against: at least one,
	// each a disease of the snapshot, none repeated.
	Diseases []string `json:"diseases"`
	// Live is the live-vaccine flag. It is required although the engine
	// does not read it before S7 (pregnancy), because an absent flag would
	// read as non-live, which is unsafe (issue #14, Q46).
	Live bool `json:"live"`
	// Retired marks a product that is no longer given (F11.3, SAD §5.6
	// table C). It stays in the snapshot with its series: a dose of it has
	// an unknown product, and it is never offered for a dose not yet given.
	Retired bool `json:"retired,omitzero"`
}

// Stage is the life stage a series belongs to.
type Stage string

// Stages.
const (
	Childhood Stage = "childhood"
	Adult     Stage = "adult"
)

// Series is a primary series for one disease. More than one plan means
// alternative plans, which all have the same number of doses (EQ5).
type Series struct {
	ID      string `json:"id"`
	Disease string `json:"disease"`
	// Product names the catalog product the series belongs to, which must
	// list Disease. Absent, the series is generic: it applies to every
	// product of the disease that has no series of its own. It is a pointer
	// so that a present empty string is a Validate problem instead of a
	// generic series. The series of one disease with the same Product, or
	// its generic series, form a product group (issue #14). A group of more
	// than one series tells them apart by StartAge (EQ26). Added in S3a.
	Product *string `json:"product,omitzero"`
	Stage   Stage   `json:"stage"`
	// StartAge is the start-age bracket of the series: the ages at dose 1
	// it applies to. Required in a product group of more than one series,
	// whose brackets may not overlap (EQ26). Added in S3b.
	StartAge *StartAge `json:"startAge,omitzero"`
	// MaxAgeStart is the exclusive maximum age at dose 1: dose 1 must be
	// given before birth + MaxAgeStart, so its last allowed day is birth +
	// MaxAgeStart − 1 day ("14 weeks 6 days" is written as its exclusive
	// bound P15W, see Span). It belongs to the series, not to a plan. The
	// engine also limits dose 1 by the last start-age bracket max of the
	// series' product group, whichever is earlier (issue #15, Q55). Added
	// in S4a.
	MaxAgeStart *Span `json:"maxAgeStart,omitzero"`
	// MaxAgeComplete is the exclusive maximum age for every dose of the
	// series: each dose must be given before birth + MaxAgeComplete. Added
	// in S4a.
	MaxAgeComplete *Span `json:"maxAgeComplete,omitzero"`
	// CatchUpAfterAge is the catch-up age: when the next dose's first
	// possible day F (the earliest date the result would show) is at or
	// after birth + CatchUpAfterAge, the engine asks for a doctor instead of
	// scheduling it (SAD §5.2, EQ26). Added in S4c.
	CatchUpAfterAge *Span `json:"catchUpAfterAge,omitzero"`
	// ValidFrom and ValidTo are the series' validity window: it is in effect
	// on day d iff (ValidFrom absent or d ≥ ValidFrom) and (ValidTo absent or
	// d < ValidTo), a half-open window like StartAge (EQ26). Validate rejects
	// an empty window. Added in S4c.
	ValidFrom *Date  `json:"validFrom,omitzero"`
	ValidTo   *Date  `json:"validTo,omitzero"`
	Plans     []Plan `json:"plans"`
}

// StartAge is a start-age bracket: the half-open range [Min, Max) of the age
// at dose 1 (SAD §5.2, F05.2, EQ26). Max absent: the bracket is open-ended
// (issue #23, Q25). Validate rejects a bracket that may be empty. Added in
// S3b.
type StartAge struct {
	Min Span  `json:"min"`
	Max *Span `json:"max,omitzero"`
}

// Plan is one alternative schedule of a series.
type Plan struct {
	ID    string     `json:"id"`
	Doses []DoseRule `json:"doses"`
}

// DoseRule holds the rules for one dose position of a plan. Every rule has a
// minimum age or a minimum interval (A7; MinIntervalFromFirst counts as one);
// dose 1 always has a minimum age and no interval.
type DoseRule struct {
	// MinAge is the minimum age at which the dose may be given.
	MinAge *Span `json:"minAge,omitzero"`
	// MinInterval is the minimum time since the previous dose in time.
	MinInterval *Span `json:"minInterval,omitzero"`
	// MinIntervalFromFirst is the minimum time since dose 1 (SAD §5.2), such
	// as HPV dose 3 at least 5 months after dose 1. Added in S2.
	MinIntervalFromFirst *Span     `json:"minIntervalFromFirst,omitzero"`
	Recommended          Milestone `json:"recommended"`
	// Buffer only moves the overdue date: overdue = due + max(buffer, 7 days).
	Buffer *Span `json:"buffer,omitzero"`
	// Tolerance is a sourced grace period, in days or weeks only: a given
	// dose meets this rule's minimums when it is at most Tolerance early
	// ((anchor + span) − tolerance). It applies only when given doses are
	// validated, never to a next-dose date or limit (F05.10, EQ27). P0D means
	// no tolerance. Added in S4c.
	Tolerance *Span `json:"tolerance,omitzero"`
}

// Anchor is the date a recommended milestone counts from.
type Anchor string

// Anchors.
const (
	FromBirth        Anchor = "birth"
	FromFirstDose    Anchor = "firstDose"
	FromPreviousDose Anchor = "previousDose"
)

// Milestone is the recommended date of a dose: anchor + offset.
type Milestone struct {
	From   Anchor `json:"from"`
	Offset Span   `json:"offset"`
}
