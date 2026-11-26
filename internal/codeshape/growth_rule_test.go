package codeshape

import (
	"strings"
	"testing"
)

// shapeAt builds one measured function for the growth-rule fixtures: name,
// package dir, and the two measurements.
func shapeAt(name, dir string, lines, cx int) FuncShape {
	return FuncShape{Name: name, File: dir + "/f.go", Line: 1, Lines: lines, Complexity: cx}
}

// TestGrowthRule covers the base-relative growth rule on fixtures: growth
// within the percentage and its floor passes, growth past it fails, a
// function that crosses a threshold is held to the same allowance from its
// base value, a new function over a threshold fails, a moved function keeps
// its base history and a renamed one starts fresh, and a function under both
// thresholds is never checked.
func TestGrowthRule(t *testing.T) {
	// Base: big is long and complex, tame is under both thresholds, cross is
	// just under both (299 lines of 300, complexity 3 of 50).
	base := []FuncShape{
		shapeAt("big", "p", 787, 60),
		shapeAt("tame", "p", 100, 10),
		shapeAt("cross", "q", 299, 3),
	}
	at := func(name, dir string, lines, cx int) []FuncShape {
		return []FuncShape{shapeAt(name, dir, lines, cx)}
	}
	// big's allowance: 5% of 787 = 39 (floor 10) -> limit 826. tame's: 5% of
	// 100 = 5, floor 10 -> limit 110. cross's: 5% of 299 = 14 -> limit 313 on
	// lines; complexity 3 -> 5% = 0, floor 2 -> a crossing function may reach
	// complexity 5 at head only.
	for _, c := range []struct {
		name   string
		head   []FuncShape
		want   int    // number of failures
		metric string // the metric of the first failure, when want > 0
	}{
		{"big within the percent", at("big", "p", 800, 60), 0, ""},
		{"big at the limit", at("big", "p", 826, 60), 0, ""},
		{"big one over the limit", at("big", "p", 827, 60), 1, "lines"},
		{"big shrank", at("big", "p", 700, 60), 0, ""},
		{"big left both thresholds", at("big", "p", 250, 40), 0, ""},
		{"floor holds a small function", at("tame", "p", 108, 10), 0, ""},
		// (The floor is exercised below against a lowered threshold: at
		// LongFuncLines 300 the lines floor never binds, since 5% of any base
		// over 300 exceeds 10.)
		{"crossing the line threshold within the allowance", at("cross", "q", 305, 3), 0, ""},
		{"crossing the line threshold past it", at("cross", "q", 314, 3), 1, "lines"},
		{"a new long function fails", at("fresh", "p", 350, 10), 1, "lines"},
		{"a new function under the threshold passes", at("fresh", "p", 299, 10), 0, ""},
		{"complexity within the allowance", at("big", "p", 787, 62), 0, ""},
		{"complexity past the allowance", at("big", "p", 787, 64), 1, "complexity"},
		{"complexity shrank under the threshold", at("big", "p", 787, 49), 0, ""},
		{"crossing the complexity threshold past its allowance", at("cross", "q", 299, 55), 1, "complexity"},
		// Same key ("p big": package dir and receiver-qualified name), another file.
		{"moved to another file of the package keeps its history", []FuncShape{{Name: "big", File: "p/g.go", Line: 9, Lines: 800, Complexity: 60}}, 0, ""},
		{"renamed starts fresh as a new long function", at("bigger", "p", 800, 40), 1, "lines"},
	} {
		fails, oks := CheckFuncGrowth(base, c.head, DefaultGrowthLimits())
		if len(fails) != c.want {
			t.Errorf("%s: %d failures (%v), want %d", c.name, len(fails), fails, c.want)
			continue
		}
		if c.want == 0 {
			// A checked-and-passing function is reported as ok; a function
			// under both thresholds is not checked at all.
			checked := false
			for _, o := range oks {
				if o.Shape.Name == c.head[0].Name {
					checked = true
				}
			}
			over := c.head[0].Lines > LongFuncLines || c.head[0].Complexity > ComplexFuncThreshold
			if over && !checked && !strings.Contains(c.name, "shrank") {
				t.Errorf("%s: over a threshold at head and passing, but not reported as ok", c.name)
			}
			if !over && checked {
				t.Errorf("%s: under both thresholds, but reported as checked", c.name)
			}
			continue
		}
		if fails[0].Metric != c.metric {
			t.Errorf("%s: failed on %q, want %q", c.name, fails[0].Metric, c.metric)
		}
	}
}

// TestGrowthRuleFloor exercises the allowances' floors against thresholds
// low enough for 5% of the base to fall under them: with the lines threshold
// at 100, a base of 100 lines has an allowance of max(5, 10) = 10 (limit
// 110); with the complexity threshold at 10, a base of 8 has max(0, 2) = 2
// (limit 10).
func TestGrowthRuleFloor(t *testing.T) {
	lim := GrowthLimits{Lines: 100, LinesPercent: GrowthPercent, LinesFloor: GrowthFloorLines,
		Complexity: 10, ComplexityPercent: ComplexGrowthPercent, ComplexityFloor: ComplexGrowthFloor}
	base := []FuncShape{shapeAt("f", "p", 100, 8)}
	for _, c := range []struct {
		name   string
		head   []FuncShape
		want   int
		metric string
	}{
		{"lines within the floor", []FuncShape{shapeAt("f", "p", 110, 8)}, 0, ""},
		{"lines one past the floor", []FuncShape{shapeAt("f", "p", 111, 8)}, 1, "lines"},
		{"complexity within the floor", []FuncShape{shapeAt("f", "p", 100, 10)}, 0, ""},
		{"complexity one past the floor", []FuncShape{shapeAt("f", "p", 100, 11)}, 1, "complexity"},
	} {
		fails, _ := CheckFuncGrowth(base, c.head, lim)
		if len(fails) != c.want || (c.want > 0 && fails[0].Metric != c.metric) {
			t.Errorf("%s: %d failures (%v), want %d on %q", c.name, len(fails), fails, c.want, c.metric)
		}
	}
}

// TestGrowthRuleSeeded pins the 11-26 batch failure that motivated the rule:
// (*cardBuilder).static in dirac/kgen/build_ab.go stood at 812 lines over a
// base row of 787 plus its slack of 23 and failed TestLongFunctionsOnlyShrink
// for the #5064 batch. Under the growth rule 812 is inside 787's allowance
// (787 + max(5%, 10) = 826) and passes; 830 fails, and the message names the
// function, its file, the base and head values and the limit.
func TestGrowthRuleSeeded(t *testing.T) {
	base := []FuncShape{shapeAt("(*cardBuilder).static", "dirac/kgen", 787, 120)}
	base[0].File = "dirac/kgen/build_ab.go"

	head := []FuncShape{shapeAt("(*cardBuilder).static", "dirac/kgen", 812, 120)}
	head[0].File = "dirac/kgen/build_ab.go"
	fails, _ := CheckFuncGrowth(base, head, DefaultGrowthLimits())
	if len(fails) != 0 {
		t.Fatalf("812 lines over a base of 787 failed; the 11-26 batch case must pass: %v", fails)
	}

	head[0].Lines = 830
	fails, _ = CheckFuncGrowth(base, head, DefaultGrowthLimits())
	if len(fails) != 1 {
		t.Fatalf("830 lines over a base of 787: %d failures, want 1", len(fails))
	}
	msg := fails[0].String()
	for _, want := range []string{
		"(*cardBuilder).static", "dirac/kgen/build_ab.go", "lines", "830", "787", "826",
		"Extract the new logic into a named helper or a registered handler",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("failure message %q does not contain %q", msg, want)
		}
	}

	// The complexity advice is its own text.
	cxBase := []FuncShape{shapeAt("(*cardBuilder).static", "dirac/kgen", 787, 100)}
	cxHead := []FuncShape{shapeAt("(*cardBuilder).static", "dirac/kgen", 787, 110)}
	cxHead[0].File = "dirac/kgen/build_ab.go"
	fails, _ = CheckFuncGrowth(cxBase, cxHead, DefaultGrowthLimits())
	if len(fails) != 1 || fails[0].Metric != "complexity" {
		t.Fatalf("complexity 110 over a base of 100: %v, want one complexity failure", fails)
	}
	if msg := fails[0].String(); !strings.Contains(msg, "Move the new branch into a handler") {
		t.Errorf("complexity failure %q does not carry the complexity advice", msg)
	}
}
