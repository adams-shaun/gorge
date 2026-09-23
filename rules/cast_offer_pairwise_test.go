package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// pairwiseCensusSrc is a mandatory TargetMin$ 2 | TargetMax$ 2
// TargetsWithSameController$ spell. Its two legal candidates below are split
// across controllers, so the candidate COUNT reaches the minimum while the
// pairwise set constraint is unsatisfiable: the cast-offer census (pure count
// feasibility) must still OFFER it, and the post-push targetAsk abort (CR
// 601.2c via CR 733.1, Note "cast aborted: no legal target") is the failure
// mode -- the same contract TestRunAwayTogetherMandatoryTwoSameControllerAbortsCast
// pins for the TargetsWithDifferentControllers$ shape.
const pairwiseCensusSrc = "Name:Pairwise Census\nManaCost:1 W\nTypes:Instant\n" +
	"A:SP$ Draw | Defined$ You | ValidTgts$ Creature.Other | TargetMin$ 2 | " +
	"TargetMax$ 2 | TargetsWithSameController$ True | Oracle:x\n"

func TestCastOfferCensusOffersPairwiseConstrainedCast(t *testing.T) {
	e, _, id := newFixtureDeck(t, 6015, pairwiseCensusSrc)
	bearA := bearPermanent(t, e, 0)
	bearB := bearPermanent(t, e, 1)
	if a, b := e.G.Obj(bearA), e.G.Obj(bearB); a == nil || b == nil || a.Zone != state.ZBattlefield || b.Zone != state.ZBattlefield || a.Controller == b.Controller {
		t.Fatalf("precondition: bears are not battlefield permanents under distinct controllers: %+v %+v", a, b)
	}
	addMana(t, e, 0, "1W")
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZHand || o.Face() == nil {
		t.Fatalf("precondition: pairwise census spell is not a face-up hand card: %+v", o)
	}
	sa := o.Face().SpellAbility()
	if sa == nil || sa.Params["TargetMin"] != "2" || sa.Params["TargetMax"] != "2" ||
		sa.Params["TargetsWithSameController"] != "True" {
		t.Fatalf("precondition: fixture lost the mandatory same-controller pair shape: %+v", sa)
	}
	candidates := e.legalTargetCandidates(0, id, id, sa)
	if len(candidates) != 2 {
		t.Fatalf("precondition: target census found %d legal candidates, want the two bears", len(candidates))
	}
	// The boundary's precondition: the count reaches the minimum but the
	// same-controller capacity does not -- exactly the shape the pairwise
	// census clause used to withhold pre-offer.
	_, _, capacity, constrained := e.sameControllerTargetBounds(sa, candidates, 2, 2)
	if !constrained || capacity != 1 {
		t.Fatalf("precondition: same-controller bound = (capacity %d, constrained %v), want constrained capacity 1", capacity, constrained)
	}
	if !castOffered(e, id) {
		t.Fatal("pairwise-constrained cast with a satisfiable candidate count was withheld -- the offer census must stay count-only so the ask's CR 733.1 abort is the failure mode")
	}
	var cast *decision.Option
	for _, opt := range e.Pending().Options {
		if opt.Kind == "cast" && opt.Obj == id {
			c := opt
			cast = &c
		}
	}
	if cast == nil {
		t.Fatal("precondition: the cast option disappeared from the priority window before submission")
	}
	submitChoices(t, e, cast.Index)
	if !hasNote(e, "cast aborted: no legal target") {
		t.Fatal("the pairwise-unsatisfiable cast did not abort at the ask")
	}
	if z := e.G.Obj(id).Zone; z != state.ZHand {
		t.Fatalf("spell zone = %v, want hand (the CR 733.1 reversal)", z)
	}
	e.Advance()
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("priority did not resume after the abort: %+v", d)
	}
}
