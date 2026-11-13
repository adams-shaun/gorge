package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPotentialPlanPricesMultikicker pins CR 702.43's potential play:
// potentialModeBaseCostMultikick prices the multikicked offer as the base
// plus ONE payment, so an uncast multikicker spell is a potential play from
// untapped sources and the count ask that follows offers 0..max with the
// right max. Without the pricing the mode is "shape:unsupported" and no
// potential play exists.
func TestPotentialPlanPricesMultikicker(t *testing.T) {
	t.Parallel()
	const src = "Name:Multikick Probe\nManaCost:2 G\nTypes:Creature\nPT:2/2\nK:Multikicker:1 G\nOracle:x\n"

	// Four Forests pay the base {2}{G} but not base + one {1}{G}: the
	// potential plan is proven insufficient, not a witness.
	e4, _, probe4 := newFixtureDeck(t, 9601, src)
	for i := 0; i < 4; i++ {
		onBoard(t, e4, 0, lrForest)
	}
	e4.G.Step = state.StepMain1
	ppPriority(t, e4)
	if got := e4.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: pool %d, want empty (untapped sources)", got)
	}
	if pp4, ok := potentialCastModeOK(e4.PotentialPaymentPlans(0), probe4, "multikicked"); ok {
		t.Fatalf("four Forests pay the base but not base + one payment, yet the multikicked play is priced: %+v", pp4)
	}

	// Five Forests pay base + one payment exactly: the plan has five
	// activations, and following it poses the count with max 1.
	e, _, probe := newFixtureDeck(t, 9602, src)
	for i := 0; i < 5; i++ {
		onBoard(t, e, 0, lrForest)
	}
	e.G.Step = state.StepMain1
	ppPriority(t, e)
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: pool %d, want empty (untapped sources)", got)
	}
	if !potentialHas(e, "cast", probe, "multikicked") {
		t.Fatalf("the multikicked cast is not a potential play: %+v", e.PotentialActions(0))
	}
	pp := potentialCastMode(t, e.PotentialPaymentPlans(0), probe, "multikicked")
	if pp.Plan == nil || pp.Reason != "" {
		t.Fatalf("five Forests: verdict %+v, want a witness", pp)
	}
	if got := len(pp.Plan.Activations); got != 5 {
		t.Fatalf("witness activates %d sources, want 5 for {2}{G} + {1}{G}", got)
	}
	for _, a := range pp.Plan.Activations {
		src := a.Source
		ppChoose(t, e, func(o decision.Option) bool {
			return o.Kind == "activate" && o.Obj == src
		})
	}
	ppChoose(t, e, func(o decision.Option) bool {
		return o.Kind == "cast" && o.Obj == probe && o.Mode == "multikicked"
	})
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 ||
		d.Options[0].Kind != "multikick" || d.Options[0].Amount != 0 ||
		d.Options[1].Amount != 1 {
		t.Fatalf("multikick ask %+v, want 0..1 from the five-mana pool", d)
	}
	submitChoices(t, e, d.Options[1].Index)
	if o := e.G.Obj(probe); o.Zone != state.ZStack || o.TimesKicked != 1 {
		t.Fatalf("on the stack: zone %s kicked %d, want the one answered payment", o.Zone, o.TimesKicked)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after payment %d, want 0 (base + one payment exactly)", got)
	}
}

// potentialCastModeOK returns the potential cast of obj in mode.
func potentialCastModeOK(pps []PotentialPlan, obj state.ObjID, mode string) (PotentialPlan, bool) {
	for _, pp := range pps {
		if pp.Action.Kind == "cast" && pp.Action.Obj == obj && pp.Action.Mode == mode {
			return pp, true
		}
	}
	return PotentialPlan{}, false
}

// potentialCastMode returns the potential cast of obj in mode, failing when
// the walk does not price it.
func potentialCastMode(t *testing.T, pps []PotentialPlan, obj state.ObjID, mode string) PotentialPlan {
	t.Helper()
	pp, ok := potentialCastModeOK(pps, obj, mode)
	if !ok {
		t.Fatalf("no potential cast of %d in mode %q in %+v", obj, mode, pps)
	}
	return pp
}
