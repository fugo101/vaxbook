package engine

// "The rule in effect on its dose date or today" (SAD §5.3 step 5, F05.10
// "Rule versions", EQ26; issue #29 In scope 3, Q77, Q93–Q96): a dated dose
// in the series of today's candidate c counts when it meets c, or a version
// of c (version) in effect on its date.

// sev is a dose verdict's severity: the worse of its minimum verdict and
// its maximum-age verdict.
func (d doseVerdict) sev() verdict {
	switch {
	case len(d.maxDef) > 0:
		return verdictDefinite
	case len(d.maxPos) > 0:
		return max(d.minV, verdictPossible)
	}
	return d.minV
}

// verdictAt is the S4a verdict of a dose on x at position pos under s at
// birth end b (Q62): the minimum checks by best plan, with s's tolerance,
// against pd and first, and s's own maximum ages.
func (s seriesModel) verdictAt(pos int, b birthEnd, x Range, pd predecessor, first *Range) doseVerdict {
	return s.splitVerdictAt(pos, b, b, x, pd, first)
}

// splitVerdictAt is verdictAt with the minimum checks read at minB and the
// maximum ages at maxB. With minB = b_old and maxB = b_late it is the most
// lenient reading of a birth range: a minimum age is reached first from the
// oldest birth date and a maximum age passed last from the latest, and the
// intervals do not read the birth date. A dose that some birth date of the
// range passes therefore passes it (Q107).
func (s seriesModel) splitVerdictAt(pos int, minB, maxB birthEnd, x Range, pd predecessor, first *Range) doseVerdict {
	v, checks := s.checkWith(pos, minB, x, pd, first, gapBound{})
	dv := doseVerdict{minV: v, minChecks: checks}
	dv.addMax(s.maxAges(pos, maxB, x))
	return dv
}

// match is how a version's bracket matches the age at dose 1 (Q93).
type match uint8

const (
	matchNone     match = iota // cannot contain it: not a version
	matchPossible              // possibly contains it: may soften a definite failure
	matchCertain               // certainly contains it: its verdict counts
)

// bracketMatch matches version v's bracket against the age at dose 1, over
// the birth range [b_old, b_late] (Q93, Q95). The bracket covers its reach
// in its own segment, [floor, max) as compiled there, and a series alone in
// its group there covers every age (C8). It certainly contains the age when
// every birth date of the range and every day of dose 1's range firstBr
// lie in the reach, and possibly when some do (bracket.mayContain). With no
// birth date a bracketed version is possible. datedVerdict always passes a
// dose-1 range (Q106).
func bracketMatch(v version, b birthEnd, firstBr Range) match {
	br := v.s.start
	switch {
	case !br.ok || v.lone:
		return matchCertain
	case !b.known:
		return matchPossible
	}
	old, late := b.older(), b.later()
	if (!br.floor.ok || !firstBr.Lo.Before(late.AddSpan(br.floor.span))) && (!br.max.ok || firstBr.Hi.Before(old.AddSpan(br.max.span))) {
		return matchCertain
	}
	if br.mayContain(firstBr, Range{Lo: old, Hi: late}) {
		return matchPossible
	}
	return matchNone
}

// datedVerdict is the verdict of a dated dose of product on x at position
// pos in the series of today's candidate s, with the versions of s
// (issue #29 In scope 3). pd and first are the walk's predecessor (today's
// chain) and dose 1, and firstBr dose 1's range for the bracket match (nil
// when no dose 1 is counted).
//
//   - c's own S4a verdict on all of x is vc. Without versions, or when vc is
//     OK, it is the verdict.
//   - x is split at the versions' segment bounds. On each part, c is judged
//     on the part at b, and each version d whose segment holds the part,
//     that applies to the dose's product (Q94, Q116) and to position pos
//     (Q96), and whose bracket matches (Q93, Q95) is judged on the part at
//     b_old and b_late (at b with no birth date), with d's rule, tolerance
//     and maximum ages from its own segment. Dose 1's range for the match is
//     the part itself for pos 1, else firstBr, else [b_old, x.Hi]: with no
//     dose 1 counted, dose 1 lies between birth and the dose being judged
//     (Q106).
//   - A certain version that passes at both ends is a certain verdict: the
//     worse of its two. The part's verdict is the best certain one (the
//     smallest severity; a tie goes to c, then rule-set order). A version
//     that passes for some birth date of the range but not at both ends, or
//     a possible version that passes for some, counts as possible (Q93,
//     Q107): when the part's verdict is definite it is demoted, every
//     definite check turning possible, and a possible version never makes a
//     part OK. "Passes for some birth date" is read leniently: at either end,
//     or with its minimums at b_old and its maximum ages at b_late
//     (splitVerdictAt), so a version that fails its minimum age at b_late and
//     its maximum age at b_old, and passes between, softens too.
//   - The parts combine to OK when all are OK, definite when all are
//     definite (their checks unioned per kind), else possible (every check
//     failed in some part). The verdict is the better of that and vc (a tie
//     goes to vc): S4a judges minimum and maximum checks apart, so without
//     vc a series re-issued with identical rules could turn a possible
//     verdict into a definite one.
//
// For a fixed position, range, predecessor and dose 1 the versions never
// make the verdict worse than vc. The work per dose is the number of cuts
// times the number of versions, a constant of the snapshot.
func (s seriesModel) datedVerdict(pos int, b birthEnd, x Range, pd predecessor, first, firstBr *Range, product string) doseVerdict {
	vc := s.verdictAt(pos, b, x, pd, first)
	if len(s.versions) == 0 || vc.sev() == verdictOK {
		return vc
	}
	ends := []birthEnd{b}
	if b.known {
		ends = []birthEnd{{date: b.older(), known: true}, {date: b.later(), known: true}}
	}
	var parts []doseVerdict
	lo := x.Lo
	for {
		part := Range{Lo: lo, Hi: x.Hi}
		next, cut := Date{}, false
		for _, c := range s.cuts {
			if c.After(lo) && !c.After(x.Hi) {
				next, cut = c, true
				break
			}
		}
		if cut {
			part.Hi = next.AddDays(-1)
		}
		best, softer := s.verdictAt(pos, b, part, pd, first), false
		for _, v := range s.versions {
			if !v.seg.contains(part.Lo) || !v.applies(product) || !v.lastDose(pos, s.length) {
				continue
			}
			var doseOne Range
			switch {
			case pos == 1:
				doseOne = part
			case firstBr != nil:
				doseOne = *firstBr
			default:
				doseOne = Range{Lo: b.older(), Hi: x.Hi}
			}
			m := bracketMatch(v, b, doseOne)
			if m == matchNone {
				continue
			}
			// worse is the certain verdict, lenient the most lenient reading
			// of the birth range.
			var worse, lenient doseVerdict
			for i, e := range ends {
				ev := v.s.verdictAt(pos, e, part, pd, first)
				if i == 0 || ev.sev() > worse.sev() {
					worse = ev
				}
				if i == 0 || ev.sev() < lenient.sev() {
					lenient = ev
				}
			}
			if len(ends) == 2 {
				if ev := v.s.splitVerdictAt(pos, ends[0], ends[1], part, pd, first); ev.sev() < lenient.sev() {
					lenient = ev
				}
			}
			if m == matchCertain && worse.sev() < best.sev() {
				best = worse
			}
			if lenient.sev() < verdictDefinite {
				softer = true
			}
		}
		if softer && best.sev() == verdictDefinite {
			best = best.demoted()
		}
		parts = append(parts, best)
		if !cut {
			break
		}
		lo = next
	}
	vp := combineParts(parts)
	if vp.sev() < vc.sev() {
		return vp
	}
	return vc
}

// combineParts combines the verdicts of the parts of a dose range: OK when
// all are OK, definite when all are definite (the definite minimum checks
// and the definite maximum-age checks each unioned), else possible, with
// every check failed in some part (Q77).
func combineParts(parts []doseVerdict) doseVerdict {
	if len(parts) == 1 {
		return parts[0]
	}
	allOK, allDef := true, true
	for _, p := range parts {
		allOK = allOK && p.sev() == verdictOK
		allDef = allDef && p.sev() == verdictDefinite
	}
	var out doseVerdict
	switch {
	case allOK:
	case allDef:
		for _, p := range parts {
			if p.minV == verdictDefinite {
				out.minV = verdictDefinite
				out.minChecks = unionChecks(out.minChecks, p.minChecks)
			}
			out.maxDef = unionChecks(out.maxDef, p.maxDef)
		}
	default:
		for _, p := range parts {
			if p.minV != verdictOK {
				out.minV = verdictPossible
				out.minChecks = unionChecks(out.minChecks, p.minChecks)
			}
			out.maxPos = unionChecks(out.maxPos, unionChecks(p.maxDef, p.maxPos))
		}
	}
	return out
}
