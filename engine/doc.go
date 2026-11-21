// Package engine computes vaccination assessments for one profile from a
// rule set snapshot (SAD §5).
//
// The engine is pure: it never reads the clock (the caller passes "today"),
// does no I/O and never coerces input values. It imports the standard
// library and package ruleset only, so the same code runs natively on the
// server and as WASM in the browser with byte-identical output (ADR-0002).
//
// Every date inside the engine is a closed range [Lo, Hi]; an exact date is
// the range with Lo == Hi. The assessment steps take one end of the
// birth-date range as a parameter (SAD §5.3 step 4).
package engine
