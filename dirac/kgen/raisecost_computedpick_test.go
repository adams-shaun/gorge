package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRaiseCostComputedPickLowers: a card's own RaiseCost Cost$ whose count is
// computed, not announced, lowers. Tectonic Split's Sac<X/Land> reads SVar X
// as Count$Valid Land.YouCtrl/HalfUp: gorge folds it into a fixed Sac<N/Land>
// where the cost is composed (raise_cost_extra.go), so no X is announced and
// the kernel counts the lands as the pick is posed (kop.Cost.SacHalf).
func TestRaiseCostComputedPickLowers(t *testing.T) {
	all, bs := costProgCastBuild(t, []string{"Tectonic Split"})
	if len(all) != 1 || len(bs) != 1 {
		t.Fatalf("lowered %d cards, built %d, want one of each", len(all), len(bs))
	}
	if got := costProgCastRouteReason(all[0]); got != "" {
		t.Fatalf("Tectonic Split refusals %q, want none", got)
	}
	c := bs[0].def.SpellCost
	if c.Flags&kop.CostSac == 0 || c.N != 0 || !c.SacHalf || c.Filter == 0 {
		t.Fatalf("Tectonic Split additional cost %+v, want CostSac with N 0, SacHalf and a land filter", c)
	}

	// the same Sac<X/Land> with another body of X is the announced X or a
	// different computed count: neither is "half the lands"
	script := func(xvar string) string {
		return xRaiseScript("4 G G", "Sac<X/Land/land(s)>", xvar)
	}
	for _, x := range []string{
		"Count$Valid Land.YouCtrl",               // all of them, not half
		"Count$Valid Land.YouCtrl/HalfDown",      // rounded the other way
		"Count$Valid Creature.YouCtrl/HalfUp",    // another type than the sacrificed
		"Count$Valid Land.OppCtrl/HalfUp",        // the opponent's lands
		"Count$Valid Land.YouCtrl/HalfUp/Plus.1", // a second operation
	} {
		if got := buildRefusals(t, "Probe X", script(x)); got == "" {
			t.Errorf("Sac<X/Land> raise with SVar X %q lowered", x)
		}
	}
	if got := buildRefusals(t, "Probe X", script("Count$Valid Land.YouCtrl/HalfUp")); got != "" {
		t.Errorf("half-the-lands raise on a probe spell refused: %q", got)
	}
}
