// PROTOTYPE — throwaway. Minimal VaxBook recommendation engine.
// Pure: no clock, no I/O. Same code runs native (server) and js/wasm (browser).
package engine

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Engine struct {
	rs       *RuleSet
	series   map[string][]*Series
	products map[string]*Product
}

func New(rs *RuleSet) (*Engine, error) {
	e := &Engine{rs: rs, series: map[string][]*Series{}, products: map[string]*Product{}}
	for i := range rs.Products {
		e.products[rs.Products[i].ID] = &rs.Products[i]
	}
	for i := range rs.Series {
		s := &rs.Series[i]
		if len(s.Plans) == 0 {
			return nil, fmt.Errorf("series %s has no plans", s.ID)
		}
		if s.Product != "" && e.products[s.Product] == nil {
			return nil, fmt.Errorf("series %s: unknown product %s", s.ID, s.Product)
		}
		e.series[s.Disease] = append(e.series[s.Disease], s)
	}
	return e, nil
}

func (e *Engine) Evaluate(in Input) Output {
	out := Output{RuleSetVersion: e.rs.Version, Today: in.Today}
	for _, d := range e.rs.Diseases {
		if len(e.series[d.ID]) == 0 {
			continue
		}
		out.Assessments = append(out.Assessments, e.evalDisease(in, d.ID))
	}
	return out
}

type doseView struct {
	rec       *DoseRecord
	lo, hi    Date
	dated     bool
	number    int
	precision Precision
}

type selection struct {
	mode   string // single | safest
	series []*Series
}

func (s selection) plans() []Plan {
	if s.mode == "single" {
		return s.series[0].Plans
	}
	var ps []Plan
	for _, x := range s.series {
		ps = append(ps, x.Plans...)
	}
	return ps
}

type result struct {
	Bucket, Block string
	next          *NextDose
	reasons       []Reason
}

func (r result) key() string {
	if r.Block != "" {
		return "block:" + r.Block
	}
	return r.Bucket
}

func (e *Engine) evalDisease(in Input, dz string) Assessment {
	a := Assessment{Disease: dz}
	var doses []doseView
	for i := range in.Doses {
		r := &in.Doses[i]
		if !contains(r.Diseases, dz) {
			continue
		}
		lo, hi, ok := r.Date.Range()
		doses = append(doses, doseView{rec: r, lo: lo, hi: hi, dated: ok, number: r.DoseNumber[dz], precision: r.Date.Precision})
	}
	a.Estimated = in.Profile.Birth.Precision != PDay
	for _, d := range doses {
		if d.precision != PDay {
			a.Estimated = true
		}
	}

	// 1. Order doses (F04.2).
	ordered, n, reasons, ok := orderDoses(doses)
	a.Reasons = append(a.Reasons, reasons...)
	if !ok {
		a.Bucket = "unknown"
		return a
	}

	// 2. Choose series (F05.3 safest series, F05.5 age-at-start ambiguity).
	bLo, bHi, hasBirth := in.Profile.Birth.Range()
	sel, selReasons := e.selectSeries(dz, ordered, bLo, bHi, hasBirth, in.Today)
	a.Reasons = append(a.Reasons, selReasons...)
	for _, s := range sel.series {
		a.SeriesUsed = append(a.SeriesUsed, s.ID)
		a.Citations = appendUnique(a.Citations, s.Citations...)
	}
	if n == 0 {
		a.Products = e.productsFor(dz, sel.series)
	}

	// F04.5 "không nhớ lịch sử thời nhỏ" — default for childhood series without own data.
	if in.Profile.HistoryStatus == "forgotChildhood" && len(doses) == 0 && allChildhood(sel.series) {
		a.Bucket = "unknown"
		a.Reasons = append(a.Reasons, Reason{Code: "FORGOT_CHILDHOOD"})
		return a
	}

	// 3. Evaluate at both ends of the birth-date range (F05.5).
	if !hasBirth {
		merge(&a, e.evalConcrete(in, sel, ordered, n, nil))
		return a
	}
	if !bLo.Before(bHi) {
		merge(&a, e.evalConcrete(in, sel, ordered, n, &bLo))
		return a
	}
	rLo := e.evalConcrete(in, sel, ordered, n, &bLo)
	rHi := e.evalConcrete(in, sel, ordered, n, &bHi)
	if rLo.key() != rHi.key() {
		hint := "birthMonth"
		if in.Profile.Birth.Precision == PMonth || rLo.Block != "" || rHi.Block != "" {
			hint = "fullBirthDate"
		}
		a.Bucket = "unknown"
		a.NextDose = rHi.next
		a.Reasons = append(a.Reasons, Reason{Code: "BIRTH_DATE_IMPRECISE", Params: map[string]string{
			"hint": hint, "atEarliestBirth": rLo.key(), "atLatestBirth": rHi.key()}})
		return a
	}
	merge(&a, rHi) // later birth end → later dates → never earlier than safe (P1)
	return a
}

func merge(a *Assessment, r result) {
	a.Bucket, a.Block, a.NextDose = r.Bucket, r.Block, r.next
	a.Reasons = append(a.Reasons, r.reasons...)
}

func orderDoses(doses []doseView) ([]doseView, int, []Reason, bool) {
	n := len(doses)
	maxNum, allNumbered := 0, len(doses) > 0
	for _, d := range doses {
		maxNum = max(maxNum, d.number)
		if d.number == 0 {
			allNumbered = false
		}
	}
	ordered := append([]doseView(nil), doses...)
	if allNumbered {
		sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].number < ordered[j].number })
	} else {
		// Level-4 doses (unknown time) do not take part in chronological ordering.
		sort.SliceStable(ordered, func(i, j int) bool {
			a, b := ordered[i], ordered[j]
			if a.dated != b.dated {
				return !a.dated
			}
			return a.dated && a.lo.Before(b.lo)
		})
		for i := 1; i < len(ordered); i++ {
			p, c := ordered[i-1], ordered[i]
			if p.dated && c.dated && !c.lo.After(p.hi) {
				return nil, 0, []Reason{{Code: "OVERLAPPING_ESTIMATED_DATES",
					Params: map[string]string{"records": p.rec.ID + "," + c.rec.ID}}}, false
			}
		}
	}
	var reasons []Reason
	if maxNum > n {
		reasons = append(reasons, Reason{Code: "IMPLIED_EARLIER_DOSES", Params: map[string]string{"count": strconv.Itoa(maxNum - n)}})
		n = maxNum
	}
	return ordered, n, reasons, true
}

func (e *Engine) selectSeries(dz string, ordered []doseView, bLo, bHi Date, hasBirth bool, today Date) (selection, []Reason) {
	var reasons []Reason
	all := e.series[dz]
	pool := all
	if len(ordered) > 0 {
		prods, anyUnknown := map[string]bool{}, false
		for _, d := range ordered {
			p := d.rec.Product
			if p == "" || e.products[p] == nil {
				anyUnknown = true
				if d.rec.FreeProduct != "" {
					reasons = append(reasons, Reason{Code: "PRODUCT_NOT_IN_CATALOG", Params: map[string]string{"name": d.rec.FreeProduct}})
				}
				continue
			}
			prods[p] = true
		}
		if !anyUnknown {
			var specific, generic []*Series
			for _, s := range all {
				if prods[s.Product] {
					specific = append(specific, s)
				}
				if s.Product == "" {
					generic = append(generic, s)
				}
			}
			if len(specific) > 0 {
				pool = specific
			} else if len(generic) > 0 {
				pool = generic
			}
		}
	}

	// Age-at-first-dose bracket.
	if hasBirth {
		var xLo, xHi Date
		have := false
		if len(ordered) == 0 {
			xLo, xHi, have = today, today, true
		} else if ordered[0].dated {
			xLo, xHi, have = ordered[0].lo, MinD(ordered[0].hi, today), true
		}
		if have {
			var f []*Series
			for _, s := range pool {
				if s.StartAge == nil || possiblyWithin(bLo, bHi, xLo, xHi, *s.StartAge) {
					f = append(f, s)
				}
			}
			if len(f) == 0 && len(ordered) == 0 {
				f = earliestBracket(pool, bLo) // too young for every bracket: future start
			}
			if len(f) > 0 {
				pool = f
			}
		}
	}

	// Group by product; ambiguous brackets within one product → series with MORE doses (F05.5).
	groups, order := map[string][]*Series{}, []string{}
	for _, s := range pool {
		if _, ok := groups[s.Product]; !ok {
			order = append(order, s.Product)
		}
		groups[s.Product] = append(groups[s.Product], s)
	}
	var chosen []*Series
	for _, p := range order {
		g := groups[p]
		if len(g) == 1 {
			chosen = append(chosen, g[0])
			continue
		}
		best := g[0]
		var ids []string
		for _, s := range g {
			ids = append(ids, s.ID)
			if maxDoses(s) > maxDoses(best) {
				best = s
			}
		}
		reasons = append(reasons, Reason{Code: "AGE_AT_START_UNCERTAIN", Params: map[string]string{
			"candidates": strings.Join(ids, ","), "chosen": best.ID}})
		chosen = append(chosen, best)
	}
	if len(chosen) > 1 {
		var ps []string
		for _, s := range chosen {
			ps = append(ps, s.Product)
		}
		reasons = append(reasons, Reason{Code: "SAFEST_SERIES", Params: map[string]string{"products": strings.Join(ps, ",")}})
		return selection{mode: "safest", series: chosen}, reasons
	}
	return selection{mode: "single", series: chosen}, reasons
}

func (e *Engine) evalConcrete(in Input, sel selection, ordered []doseView, n int, b *Date) result {
	today := in.Today
	var r result
	unknown := func(code string, params map[string]string) result {
		r.Bucket = "unknown"
		r.reasons = append(r.reasons, Reason{Code: code, Params: params})
		return r
	}
	plans := sel.plans()
	count := 0
	for _, p := range plans {
		count = max(count, len(p.Doses))
	}

	if b == nil && n == 0 {
		return unknown("NO_BIRTH_DATE", nil)
	}

	// F05.10 validity of recorded doses.
	if rs := validate(plans, sel.mode, ordered, b); len(rs) > 0 {
		r.Bucket = "unknown"
		r.reasons = rs
		return r
	}

	if n >= count {
		r.Bucket = "complete"
		r.reasons = append(r.reasons, Reason{Code: "COMPLETE"})
		return r
	}
	k := n + 1

	var prev, first *Date
	if len(ordered) > 0 {
		last := ordered[len(ordered)-1]
		if !last.dated || hasUndatedUnnumbered(ordered) {
			return unknown("PREVIOUS_DOSE_DATE_UNKNOWN", nil)
		}
		p := MinD(last.hi, today) // F04.2: min(end of period, today)
		prev = &p
		if ordered[0].dated {
			f := MinD(ordered[0].hi, today)
			first = &f
		}
	}

	nc, ok, why := nextFor(plans, sel.mode, k, b, first, prev)
	if !ok {
		return unknown(why, nil)
	}
	r.next = &NextDose{Number: k, Earliest: ptr(nc.earliest), Due: ptr(nc.due), Overdue: ptr(nc.overdue)}
	if sel.mode == "single" && len(plans) > 1 {
		var ls []string
		for _, p := range plans {
			ls = append(ls, p.Label)
		}
		r.reasons = append(r.reasons, Reason{Code: "ALTERNATIVE_PLANS", Params: map[string]string{"plans": strings.Join(ls, " | ")}})
	}

	// F03.6 option 1: milestone due before the user chose "Đã có mũi tiêm, tôi sẽ nhập".
	if in.Profile.HistoryStatus == "incomplete" && !in.Profile.HistoryStatusAt.IsZero() && nc.due.Before(in.Profile.HistoryStatusAt) {
		return unknown("NOT_ENTERED", nil)
	}

	// Max age (F05.4a, P1): judged per candidate series; disagreement → ⚪.
	if b == nil {
		if anyMaxAge(sel.series) {
			return unknown("NO_BIRTH_DATE", nil)
		}
	} else {
		var exceeded []string
		for _, s := range sel.series {
			feasible := today
			if snc, ok, _ := nextFor(s.Plans, "single", k, b, first, prev); ok {
				feasible = MaxD(today, snc.earliest)
			}
			ex := (n == 0 && s.MaxAgeStart != nil && !feasible.Before(s.MaxAgeStart.AddTo(*b))) ||
				(s.MaxAgeComplete != nil && !feasible.Before(s.MaxAgeComplete.AddTo(*b)))
			if ex {
				exceeded = append(exceeded, s.ID)
			}
		}
		if len(exceeded) > 0 && len(exceeded) == len(sel.series) {
			r.Block = "outOfAge"
			r.reasons = append(r.reasons, Reason{Code: "OUT_OF_AGE"})
			return r
		}
		if len(exceeded) > 0 {
			return unknown("CHECK_PRODUCT_NAME", map[string]string{"exceededIn": strings.Join(exceeded, ",")})
		}
	}

	// Buckets (F05.4).
	plus30, plus6m := today.AddDays(30), today.AddMonths(6)
	switch {
	case !today.Before(nc.overdue):
		r.Bucket = "overdue"
	case !today.Before(nc.earliest) && !nc.due.After(plus30):
		r.Bucket = "due"
	case nc.due.After(plus30) && !nc.due.After(plus6m):
		r.Bucket = "upcoming"
	case today.Before(nc.earliest) && !nc.due.After(plus30):
		r.Bucket = "upcoming"
	case nc.due.After(plus6m) && n > 0:
		r.Bucket = "complete"
		r.reasons = append(r.reasons, Reason{Code: "ON_TRACK", Params: map[string]string{"due": nc.due.String()}})
	case nc.due.After(plus6m):
		r.Bucket = "notYet" // SPEC GAP: no dose yet and first milestone > 6 months away
		r.reasons = append(r.reasons, Reason{Code: "NOT_YET"})
	default:
		return unknown("UNCLASSIFIED", nil)
	}
	return r
}

type nextCalc struct{ earliest, due, overdue Date }

// nextFor computes dates for dose k. Alternatives (single mode, several plans): due = earliest plan
// milestone, overdue = latest. Safest mode: every dimension takes the most conservative value.
func nextFor(plans []Plan, mode string, k int, b, first, prev *Date) (nextCalc, bool, string) {
	var nc nextCalc
	var minDue, maxRec Date
	got := false
	for _, p := range plans {
		if len(p.Doses) < k {
			continue
		}
		dr := p.Doses[k-1]
		earliest, has := earliestFor(dr, b, first, prev)
		var rec Date
		switch dr.Recommended.From {
		case "birth":
			if b == nil {
				return nc, false, "NO_BIRTH_DATE"
			}
			rec = dr.Recommended.Offset.AddTo(*b)
		case "firstDose":
			if first == nil {
				return nc, false, "PREVIOUS_DOSE_DATE_UNKNOWN"
			}
			rec = dr.Recommended.Offset.AddTo(*first)
		default: // previousDose
			if prev == nil {
				return nc, false, "PREVIOUS_DOSE_DATE_UNKNOWN"
			}
			rec = dr.Recommended.Offset.AddTo(*prev)
		}
		if !has {
			earliest = rec
		}
		due := MaxD(earliest, rec)
		over := due
		if dr.Buffer != nil {
			over = dr.Buffer.AddTo(due)
		}
		if !got {
			nc, minDue, maxRec, got = nextCalc{earliest, due, over}, due, rec, true
			continue
		}
		nc.earliest = MaxD(nc.earliest, earliest)
		nc.overdue = MaxD(nc.overdue, over)
		minDue, maxRec = MinD(minDue, due), MaxD(maxRec, rec)
	}
	if !got {
		return nc, false, "NO_RULE_FOR_DOSE"
	}
	if mode == "safest" {
		nc.due = MaxD(nc.earliest, maxRec)
	} else {
		nc.due = MaxD(nc.earliest, minDue)
	}
	nc.overdue = MaxD(nc.overdue, nc.due)
	return nc, true, ""
}

func earliestFor(dr DoseRule, b, first, prev *Date) (Date, bool) {
	var e Date
	has := false
	bump := func(d Date) {
		if !has || d.After(e) {
			e, has = d, true
		}
	}
	if dr.MinAge != nil && b != nil {
		bump(dr.MinAge.AddTo(*b))
	}
	if dr.MinInterval != nil && prev != nil {
		bump(dr.MinInterval.AddTo(*prev))
	}
	if dr.MinIntervalFromFirst != nil && first != nil {
		bump(dr.MinIntervalFromFirst.AddTo(*first))
	}
	return e, has
}

// validate checks each dated record against min age / min interval at both ends of its date range.
func validate(plans []Plan, mode string, ordered []doseView, b *Date) []Reason {
	var prevLo, prevHi, firstLo, firstHi *Date
	for i, d := range ordered {
		j := i + 1
		if d.number > 0 {
			j = d.number
		}
		if !d.dated {
			prevLo, prevHi = nil, nil
			continue
		}
		worst := earliestAcross(plans, mode, j, b, firstHi, prevHi) // latest possible previous dose
		best := earliestAcross(plans, mode, j, b, firstLo, prevLo)  // earliest possible previous dose
		params := map[string]string{"dose": strconv.Itoa(j), "record": d.rec.ID}
		if best != nil && d.hi.Before(*best) {
			return []Reason{{Code: "DOSE_INVALID", Params: params}}
		}
		if worst != nil && d.lo.Before(*worst) {
			return []Reason{{Code: "DOSE_POSSIBLY_INVALID", Params: params}}
		}
		lo, hi := d.lo, d.hi
		prevLo, prevHi = &lo, &hi
		if firstLo == nil {
			firstLo, firstHi = &lo, &hi
		}
	}
	return nil
}

func earliestAcross(plans []Plan, mode string, j int, b, first, prev *Date) *Date {
	var out *Date
	for _, p := range plans {
		if len(p.Doses) < j {
			continue
		}
		e, has := earliestFor(p.Doses[j-1], b, first, prev)
		if !has {
			continue
		}
		switch {
		case out == nil:
			out = ptr(e)
		case mode == "safest" && e.After(*out): // safest: strictest
			out = ptr(e)
		case mode != "safest" && e.Before(*out): // alternatives: valid under any plan
			out = ptr(e)
		}
	}
	return out
}

func possiblyWithin(bLo, bHi, xLo, xHi Date, r AgeRange) bool {
	return !xHi.Before(r.Min.AddTo(bLo)) && xLo.Before(r.Max.AddTo(bHi))
}

func earliestBracket(pool []*Series, bLo Date) []*Series {
	var best *Series
	for _, s := range pool {
		if s.StartAge == nil {
			return []*Series{s}
		}
		if best == nil || s.StartAge.Min.AddTo(bLo).Before(best.StartAge.Min.AddTo(bLo)) {
			best = s
		}
	}
	if best == nil {
		return nil
	}
	return []*Series{best}
}

func (e *Engine) productsFor(dz string, ss []*Series) []string {
	var out []string
	for _, s := range ss {
		if s.Product != "" {
			out = appendUnique(out, s.Product)
			continue
		}
		for _, p := range e.rs.Products {
			if contains(p.Diseases, dz) {
				out = appendUnique(out, p.ID)
			}
		}
	}
	return out
}

func maxDoses(s *Series) int {
	m := 0
	for _, p := range s.Plans {
		m = max(m, len(p.Doses))
	}
	return m
}

func allChildhood(ss []*Series) bool {
	for _, s := range ss {
		if s.Stage != "childhood" {
			return false
		}
	}
	return len(ss) > 0
}

func anyMaxAge(ss []*Series) bool {
	for _, s := range ss {
		if s.MaxAgeStart != nil || s.MaxAgeComplete != nil {
			return true
		}
	}
	return false
}

func hasUndatedUnnumbered(ds []doseView) bool {
	for _, d := range ds {
		if !d.dated && d.number == 0 {
			return true
		}
	}
	return false
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func appendUnique(xs []string, vs ...string) []string {
	for _, v := range vs {
		if !contains(xs, v) {
			xs = append(xs, v)
		}
	}
	return xs
}

func ptr(d Date) *Date { return &d }
