package rules

import "testing"

// TestManaOnlyFeeFundsPotentialCast: a mana ability whose cost is mana alone
// (Energy Refractor's "{2}: Add one mana of any color", Nomadic Elf's
// "{1}{G}: ...", Orochi Leafcaller's "{G}: ...") can fund a cast after two
// (or one) other sources pay its fee. gorge's planner census misses the
// ability (a paid gap), so the cast is offered only through the potential
// prefix search; the census must still report the ability's fee as the
// search's funding (paymentPlanCensus.fund) or the search taps no source and
// the cast is never offered (the defect this test holds).
func TestManaOnlyFeeFundsPotentialCast(t *testing.T) {
	for _, c := range []struct {
		name  string
		board []string
	}{
		{"energy refractor", []string{"Forest", "Plains", "Energy Refractor"}},
		{"nomadic elf", []string{"Forest", "Plains", "Nomadic Elf"}},
		{"orochi leafcaller", []string{"Forest", "Orochi Leafcaller"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := feeFilterBoard(t, c.board, []string{"Lightning Bolt"})
			bolt := boundHandID(t, e, "Lightning Bolt")
			plans := e.PotentialPaymentPlans(0)
			var plan *PotentialPlan
			for i := range plans {
				if plans[i].Action.Obj == bolt {
					plan = &plans[i]
					break
				}
			}
			if plan == nil {
				t.Fatalf("Lightning Bolt is not among the potential plays")
			}
			if plan.Reason != "" {
				t.Fatalf("Lightning Bolt plan reason %q detail %q, want a payable plan", plan.Reason, plan.Detail)
			}
			if len(plan.Script) == 0 {
				t.Fatalf("Lightning Bolt plan has no script: the %s fee funded no search", c.name)
			}
		})
	}
}
