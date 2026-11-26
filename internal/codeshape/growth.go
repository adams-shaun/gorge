package codeshape

import (
	"fmt"
	"sort"
)

// The per-function growth rule (operator 2026-11-26): instead of per-function
// ceiling rows shared between pull requests (the old rows-file ratchets,
// deleted), each function is held against its own size at the merge base. A
// function
// over a threshold at head may have grown by a percentage of its size at
// base, with a floor so a small function is not jumpy; a function newly over
// a threshold, or one that crosses it, is refused. The operator tunes the
// numbers through these constants (the tool's flags override them per run).

// GrowthPercent is the share of its size at base a function may grow by.
const GrowthPercent = 5

// GrowthFloorLines is the smallest line allowance, so a function just over
// LongFuncLines is not held to a one-line jitter.
const GrowthFloorLines = 10

// ComplexGrowthPercent is the share of its complexity at base a function may
// grow by.
const ComplexGrowthPercent = 5

// ComplexGrowthFloor is the smallest complexity allowance.
const ComplexGrowthFloor = 2

// GrowthLimits is the growth rule's numbers: the two thresholds and, per
// metric, the percentage and floor of the allowance.
type GrowthLimits struct {
	Lines, LinesPercent, LinesFloor                int
	Complexity, ComplexityPercent, ComplexityFloor int
}

// DefaultGrowthLimits returns the rule the growth tool runs: functions over
// LongFuncLines lines or ComplexFuncThreshold complexity, 5% growth with
// floors of 10 lines and 2 complexity.
func DefaultGrowthLimits() GrowthLimits {
	return GrowthLimits{
		Lines:             LongFuncLines,
		LinesPercent:      GrowthPercent,
		LinesFloor:        GrowthFloorLines,
		Complexity:        ComplexFuncThreshold,
		ComplexityPercent: ComplexGrowthPercent,
		ComplexityFloor:   ComplexGrowthFloor,
	}
}

// growthMetric is one of the two measures the rule holds.
type growthMetric struct {
	name      string // "lines" or "complexity"
	threshold int
	pct       int
	floor     int
	measure   func(FuncShape) int
	advice    string
}

// growthMetrics returns the rule's two metrics from lim.
func growthMetrics(lim GrowthLimits) []growthMetric {
	return []growthMetric{
		{
			name: "lines", threshold: lim.Lines, pct: lim.LinesPercent, floor: lim.LinesFloor,
			measure: func(s FuncShape) int { return s.Lines },
			advice: "Extract the new logic into a named helper or a registered handler instead of " +
				"growing the function.",
		},
		{
			name: "complexity", threshold: lim.Complexity, pct: lim.ComplexityPercent, floor: lim.ComplexityFloor,
			measure: func(s FuncShape) int { return s.Complexity },
			advice: "Move the new branch into a handler (a table entry, a per-op function registered in init, " +
				"or a helper) instead of another case or condition in the function.",
		},
	}
}

// GrowthFailure is one function the growth rule reports: over a threshold at
// head, past its allowance (a failure) or within it (an ok).
type GrowthFailure struct {
	Shape                FuncShape // the function at head (head file and line)
	Metric               string    // "lines" or "complexity"
	Base                 int       // the measurement at base; unset when New
	Head                 int
	Limit                int  // the largest head value the rule allows
	AllowPct, AllowFloor int  // the limit's make-up, as rendered by String
	New                  bool // no function of that key at the base
	advice               string
}

// String renders the report: the function, its head file and line, the base
// and head values, the limit and its make-up, and the advice.
func (f GrowthFailure) String() string {
	s := f.Shape
	if f.New {
		return fmt.Sprintf("%s (%s:%d) is a new function over %s %d at head (no function of that name and "+
			"receiver in that package at the base; the threshold is %d). %s",
			s.Name, s.File, s.Line, f.Metric, f.Head, f.Limit, f.advice)
	}
	return fmt.Sprintf("%s (%s:%d) grew to %s %d from %d at base, over its allowed limit of %d "+
		"(base %d + max(%d%% of %d, floor %d)). %s",
		s.Name, s.File, s.Line, f.Metric, f.Head, f.Base, f.Limit, f.Base, f.AllowPct, f.Base,
		f.AllowFloor, f.advice)
}

// growthReport builds the report for one head function on one metric, given
// its base value and whether a base function of the key exists.
func growthReport(s FuncShape, m growthMetric, head, base int, isNew bool) GrowthFailure {
	f := GrowthFailure{Shape: s, Metric: m.name, Head: head, New: isNew, Limit: m.threshold,
		AllowPct: m.pct, AllowFloor: m.floor, advice: m.advice}
	if !isNew {
		allow := base * m.pct / 100
		if allow < m.floor {
			allow = m.floor
		}
		f.Base, f.Limit = base, base+allow
	}
	return f
}

// CheckFuncGrowth holds the head functions to the growth rule against base
// and returns the failures, then the functions over a threshold at head that
// passed (both sorted by package, name, metric; one entry at most per metric
// per function). A function at or below both thresholds at head is never
// checked: shrinking always passes, and a function under both thresholds at
// base and head is invisible to the rule. A function over a threshold at head
// with no function of its key at the base (new, renamed, or moved across
// packages) is a failure.
func CheckFuncGrowth(base, head []FuncShape, lim GrowthLimits) (fails, oks []GrowthFailure) {
	baseByKey, _ := shapesByKey(base)
	headByKey, headKeys := shapesByKey(head)
	metrics := growthMetrics(lim)
	for _, k := range headKeys {
		s := headByKey[k]
		baseShape, ok := baseByKey[k]
		for _, m := range metrics {
			h := m.measure(s)
			if h <= m.threshold {
				continue // at or below the threshold: the rule does not apply
			}
			f := growthReport(s, m, h, m.measure(baseShape), !ok)
			if f.New || h > f.Limit {
				fails = append(fails, f)
			} else {
				oks = append(oks, f)
			}
		}
	}
	sortReports := func(list []GrowthFailure) {
		sort.Slice(list, func(i, j int) bool {
			a, b := list[i], list[j]
			if a.Shape.Key() != b.Shape.Key() {
				return a.Shape.Key() < b.Shape.Key()
			}
			return a.Metric < b.Metric
		})
	}
	sortReports(fails)
	sortReports(oks)
	return fails, oks
}
