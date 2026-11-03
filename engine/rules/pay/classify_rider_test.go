package pay

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/cost"
)

// A hybrid pip is two colours, so a may-play rider that lets any mana pay a
// coloured pip pays it too: the plan funds it as one generic and the cast is
// not declined as "cost:hybrid" (Pond Prophet under Cruelclaw's Heist).
func TestPlanManaUnderRiderFoldsHybridPips(t *testing.T) {
	t.Parallel()
	hyb := cost.Cost{Hybrid: []cost.ManaPair{{A: 'G', B: 'U'}, {A: 'G', B: 'U'}}}
	if PlanCostDetail(hyb) != "cost:hybrid" {
		t.Fatal("a hybrid cost with no rider should still decline")
	}
	for _, r := range []PipRider{{AnyType: true}, {AnyColor: true}} {
		got := PlanManaUnderRider(hyb, r)
		if got.Generic != 2 || len(got.Hybrid) != 0 {
			t.Fatalf("rider %+v: generic %d, hybrid %v, want 2 generic and no hybrid pips", r, got.Generic, got.Hybrid)
		}
		if d := PlanCostDetail(got); d != "" {
			t.Fatalf("rider %+v: the folded cost declines with %q", r, d)
		}
	}
	if got := PlanManaUnderRider(hyb, PipRider{}); len(got.Hybrid) != 2 {
		t.Fatal("no rider: the hybrid pips must stay")
	}
}
