package rules

import (
	"testing"
)

// TestPotentialPlotPlanSpendsNonSpellRestrictedMana is the autopay regression
// for a plot planned as a spell: Heartwood Crafter's {C} says it cannot be
// spent to cast spells from hand, and a plot is a CR 116.2k special action,
// not a spell (CR 702.170b), so the plan must be able to spend it. Outcaster
// Trailblazer's Plot {2}{G} can only be paid here with the Crafter's {C}: the
// Island and Forest make the other two mana.
func TestPotentialPlotPlanSpendsNonSpellRestrictedMana(t *testing.T) {
	t.Parallel()
	trailblazer := corpusCard(t, "Outcaster Trailblazer")
	e := handEngine(t, trailblazer)
	putOnBattlefield(t, e, 0, corpusCard(t, "Island"))
	putOnBattlefield(t, e, 0, corpusCard(t, "Forest"))
	crafter := putOnBattlefield(t, e, 0, corpusCard(t, "Heartwood Crafter"))

	// Precondition: the plot is a legal action at this board and the walk
	// prices it against the hypothetical pool, so the planner below is
	// asked about the play the divergence reported.
	pm := e.PotentialMana(0)
	opts := e.legalActionsPriced(0, &pm)
	if findMode(opts, "plot") < 0 {
		t.Fatalf("the plot action was never offered: %+v", opts)
	}

	var plot *PotentialPlan
	plans := e.PotentialPaymentPlans(0)
	for i := range plans {
		if plans[i].Action.Mode == "plot" {
			plot = &plans[i]
			break
		}
	}
	if plot == nil {
		t.Fatal("the potential walk has no plot plan")
	}
	if plot.Reason != "" || plot.Plan == nil {
		t.Fatalf("plot plan Reason=%q Detail=%q Plan=%v, want payable with a witness", plot.Reason, plot.Detail, plot.Plan)
	}
	for _, a := range plot.Plan.Activations {
		if a.Source == crafter {
			return
		}
	}
	t.Fatalf("plot plan %+v never activates the Crafter whose {C} pays it", plot.Plan.Activations)
}
