package codeshape

import "fmt"

// The tree-wide count growth rule (operator 2026-11-26): instead of stored
// absolute ceilings shared between pull requests (the old ratchet-test
// constants, deleted — two parallel pull requests raced the same constants
// and a stack of them failed together), each count metric is held against
// its own value at the merge base. A metric at the head may not have grown
// by more than CountGrowthPercent of its value at base, with a floor of
// CountGrowthFloor so a small count is not jumpy; the merge-base value is
// recomputed where the rule runs, never stored. A metric absent at the
// merge base (a new scope, or a census that reached zero) is held to the
// floor alone. Shrinking always passes.
//
// The rule runs in cmd/countgrowth (make lint and ci.yml's lint job), not
// as a Go test reading a stored number; TestGrowthRuleMetrics covers the
// rule itself on fixtures.

// CountGrowthPercent is the share of its value at base a count metric may
// grow by.
const CountGrowthPercent = 2

// CountGrowthFloor is the smallest count allowance, so a small count is not
// held to a one-unit jitter and a metric absent at the base is not free.
const CountGrowthFloor = 3

// CountSample is one metric's measurement: its name, its value at the merge
// base and at the head, and the advice printed when it grows past its
// allowance.
type CountSample struct {
	Name   string
	Base   int // the value at the merge base (0: absent there)
	Head   int
	Advice string
}

// CountFailure is one metric the count growth rule reports.
type CountFailure struct {
	Name                 string
	Base, Head, Limit    int
	AllowPct, AllowFloor int
	Advice               string
}

// String renders the report: the metric, its head and base values, the
// limit and its make-up, and the advice.
func (f CountFailure) String() string {
	if f.Base == 0 {
		return fmt.Sprintf("%s: measured %d at head with no value at the merge base (a metric absent at the "+
			"base, or a census that reached zero, is held to its floor of %d alone).\n%s",
			f.Name, f.Head, f.AllowFloor, f.Advice)
	}
	return fmt.Sprintf("%s: measured %d at head, %d at base, over its allowed limit of %d "+
		"(base %d + max(%d%% of %d, floor %d)).\n%s",
		f.Name, f.Head, f.Base, f.Limit, f.Base, f.AllowPct, f.Base, f.AllowFloor, f.Advice)
}

// CountAllowance is the growth allowed over base: pct of it, at least floor.
// A base of zero (a metric absent at the merge base, or a census at zero)
// allows the floor alone.
func CountAllowance(base, pct, floor int) int {
	allow := base * pct / 100
	if allow < floor {
		allow = floor
	}
	return allow
}

// CheckCountGrowth judges the samples against the growth rule and returns
// the failures, then the metrics that grew within their allowance (both in
// table order). A metric that shrank or held its value is in neither list:
// shrinking always passes and equality is not growth.
func CheckCountGrowth(samples []CountSample, pct, floor int) (fails, oks []CountFailure) {
	for _, s := range samples {
		limit := s.Base + CountAllowance(s.Base, pct, floor)
		f := CountFailure{Name: s.Name, Base: s.Base, Head: s.Head, Limit: limit,
			AllowPct: pct, AllowFloor: floor, Advice: s.Advice}
		switch {
		case s.Head > limit:
			fails = append(fails, f)
		case s.Head > s.Base:
			oks = append(oks, f)
		}
	}
	return fails, oks
}
