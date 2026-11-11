// PROTOTYPE — throwaway. Typed rule data (Q3) + engine input/output (Q14: reason codes, no prose).
package engine

// ---------- Rule set snapshot (public part) ----------

type RuleSet struct {
	Version   string     `json:"version"`
	Note      string     `json:"note,omitempty"`
	Diseases  []Disease  `json:"diseases"`
	Products  []Product  `json:"products"`
	Series    []Series   `json:"series"`
	Citations []Citation `json:"citations"`
}

type Disease struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Product struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Diseases []string `json:"diseases"`
	Live     bool     `json:"live,omitempty"`
}

type Citation struct {
	ID      string `json:"id"`
	Source  string `json:"source"`
	Locator string `json:"locator"`
}

// Series = phác đồ for one disease, optionally bound to one product and an age-at-first-dose bracket.
type Series struct {
	ID             string    `json:"id"`
	Disease        string    `json:"disease"`
	Product        string    `json:"product,omitempty"` // "" = applies to any product of the disease
	Stage          string    `json:"stage"`             // childhood | adult
	StartAge       *AgeRange `json:"startAge,omitempty"`
	MaxAgeStart    *Span     `json:"maxAgeStart,omitempty"`
	MaxAgeComplete *Span     `json:"maxAgeComplete,omitempty"`
	Plans          []Plan    `json:"plans"` // >1 = alternative plans (phác đồ thay thế)
	Citations      []string  `json:"citations"`
}

// AgeRange is [Min, Max): age at the first dose.
type AgeRange struct {
	Min Span `json:"min"`
	Max Span `json:"max"`
}

type Plan struct {
	ID    string     `json:"id"`
	Label string     `json:"label"`
	Doses []DoseRule `json:"doses"`
}

type DoseRule struct {
	MinAge               *Span  `json:"minAge,omitempty"`
	MinInterval          *Span  `json:"minInterval,omitempty"`          // from previous dose
	MinIntervalFromFirst *Span  `json:"minIntervalFromFirst,omitempty"` // from dose 1
	Recommended          Anchor `json:"recommended"`
	Buffer               *Span  `json:"buffer,omitempty"` // khoảng đệm quá hạn; default 0
}

// Anchor: recommended milestone = From + Offset. From: birth | firstDose | previousDose.
type Anchor struct {
	From   string `json:"from"`
	Offset Span   `json:"offset"`
}

// ---------- Input ----------

type Input struct {
	Today   Date         `json:"today"` // civil date in Asia/Ho_Chi_Minh, supplied by caller
	Profile Profile      `json:"profile"`
	Doses   []DoseRecord `json:"doses"`
}

type Profile struct {
	Birth           FuzzyDate `json:"birth"`
	Sex             string    `json:"sex,omitempty"`
	HistoryStatus   string    `json:"historyStatus"` // incomplete | forgotChildhood | none | complete
	HistoryStatusAt Date      `json:"historyStatusAt"`
}

type DoseRecord struct {
	ID          string         `json:"id"`
	Diseases    []string       `json:"diseases"`
	Product     string         `json:"product,omitempty"` // "" = không rõ sản phẩm
	FreeProduct string         `json:"freeProduct,omitempty"`
	Date        FuzzyDate      `json:"date"`
	DoseNumber  map[string]int `json:"doseNumber,omitempty"` // per disease, user-entered
}

// ---------- Output ----------

type Output struct {
	RuleSetVersion string       `json:"ruleSetVersion"`
	Today          Date         `json:"today"`
	Assessments    []Assessment `json:"assessments"`
}

type Assessment struct {
	Disease    string    `json:"disease"`
	Bucket     string    `json:"bucket,omitempty"` // overdue | due | upcoming | unknown | complete | notYet(spec gap)
	Block      string    `json:"block,omitempty"`  // outOfAge
	NextDose   *NextDose `json:"nextDose,omitempty"`
	SeriesUsed []string  `json:"seriesUsed,omitempty"`
	Products   []string  `json:"products,omitempty"`
	Estimated  bool      `json:"estimated,omitempty"`
	Reasons    []Reason  `json:"reasons,omitempty"`
	Citations  []string  `json:"citations,omitempty"`
}

// NextDose identity = (profile, disease, Number) — glossary "Next dose".
type NextDose struct {
	Number   int   `json:"number"`
	Earliest *Date `json:"earliest,omitempty"`
	Due      *Date `json:"due,omitempty"`
	Overdue  *Date `json:"overdue,omitempty"`
}

type Reason struct {
	Code   string            `json:"code"`
	Params map[string]string `json:"params,omitempty"`
}
