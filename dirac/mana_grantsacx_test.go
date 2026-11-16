package dirac

import "testing"

// TestGrantSacrificedCountsASacXStep: a path holding a stepSacX step is one
// that sacrificed a permanent (Ironworks' or Bolg's Company's candidate), so
// visit's skip of a candidate the path removed runs and no later step taps a
// permanent that is gone (mana_grantedoffers.go visit).
func TestGrantSacrificedCountsASacXStep(t *testing.T) {
	var g grantSrch
	g.path[0] = grantStep{gi: stepSacX, cand: 3}
	g.np = 1
	if !g.sacrificed() {
		t.Fatal("a stepSacX step does not count as a sacrifice")
	}
	g.path[0] = grantStep{gi: 0}
	if g.sacrificed() {
		t.Fatal("a plain granted tap counts as a sacrifice")
	}
}
