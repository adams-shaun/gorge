package codeshape

import "testing"

// TestGrowthRuleMetrics covers the tree-wide count growth rule on fixtures:
// growth within the percentage and its floor passes, growth past it fails, a
// small count is held to the floor, a metric absent at the merge base (or a
// census that reached zero) is held to the floor alone, and shrinking always
// passes.
func TestGrowthRuleMetrics(t *testing.T) {
	// Base 200: the allowance is 2% of 200 = 4, limit 204. Base 50: 2% of 50
	// = 1, under the floor of 3, so the floor holds and the limit is 53.
	// Base 0: a metric absent at the merge base (or a census at zero) is
	// held to the floor alone, limit 3.
	for _, c := range []struct {
		name  string
		base  int
		head  int
		limit int // the limit the rule must compute
		fail  bool
	}{
		{"holding the value passes", 200, 200, 204, false},
		{"shrinking passes", 200, 180, 204, false},
		{"growth within the percent passes", 200, 204, 204, false},
		{"growth one over the percent fails", 200, 205, 204, true},
		{"a small count is held to the floor within it", 50, 53, 53, false},
		{"a small count one over the floor fails", 50, 54, 53, true},
		{"a metric absent at the base is held to the floor within it", 0, 3, 3, false},
		{"a metric absent at the base one over the floor fails", 0, 4, 3, true},
	} {
		samples := []CountSample{{Name: "m", Base: c.base, Head: c.head, Advice: "advice"}}
		fails, oks := CheckCountGrowth(samples, CountGrowthPercent, CountGrowthFloor)
		if c.fail {
			if len(fails) != 1 || fails[0].Limit != c.limit {
				t.Errorf("%s: fails = %+v, want one failure with limit %d", c.name, fails, c.limit)
			} else if len(oks) != 0 {
				t.Errorf("%s: also reported %d ok metrics", c.name, len(oks))
			} else if fails[0].Advice != "advice" {
				t.Errorf("%s: the failure does not carry the advice", c.name)
			}
			continue
		}
		if len(fails) != 0 {
			t.Errorf("%s: fails = %+v, want none", c.name, fails)
		}
		if c.head > c.base && (len(oks) != 1 || oks[0].Limit != c.limit) {
			t.Errorf("%s: oks = %+v, want one ok with limit %d", c.name, oks, c.limit)
		}
	}

	// The failure report names the metric, its base and head values, the
	// limit and its make-up.
	fails, _ := CheckCountGrowth([]CountSample{{Name: "engineMethodCount", Base: 2423, Head: 2490, Advice: "advice text"}},
		CountGrowthPercent, CountGrowthFloor)
	if len(fails) != 1 {
		t.Fatalf("fails = %+v, want one", fails)
	}
	want := "engineMethodCount: measured 2490 at head, 2423 at base, over its allowed limit of 2471 " +
		"(base 2423 + max(2% of 2423, floor 3)).\nadvice text"
	if got := fails[0].String(); got != want {
		t.Errorf("report = %q, want %q", got, want)
	}

	// And a metric absent at the base names the floor alone.
	fails, _ = CheckCountGrowth([]CountSample{{Name: "newScope: strings.* calls", Base: 0, Head: 9, Advice: "advice text"}},
		CountGrowthPercent, CountGrowthFloor)
	want = "newScope: strings.* calls: measured 9 at head with no value at the merge base (a metric absent at the " +
		"base, or a census that reached zero, is held to its floor of 3 alone).\nadvice text"
	if len(fails) != 1 || fails[0].String() != want {
		t.Errorf("absent-at-base report = %+v, want %q", fails, want)
	}
}
