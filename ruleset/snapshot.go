package ruleset

// Snapshot is an immutable rule set version: the S1 subset of SAD §5.2.
// Fields are added additively by later slices.
type Snapshot struct {
	Version string `json:"version"`
	// Illustrative marks a fixture or demo rule set, not medically verified;
	// the publish path never sets it.
	Illustrative bool      `json:"illustrative,omitzero"`
	Diseases     []Disease `json:"diseases"`
	Series       []Series  `json:"series"`
}

// Disease is a disease the engine assesses, in display order.
type Disease struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
	Stage   Stage  `json:"stage"`
	Plans   []Plan `json:"plans"`
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
