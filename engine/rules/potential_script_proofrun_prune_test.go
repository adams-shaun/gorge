package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The nodePlans proof run (PotentialPlayScript's census-gap walk) returns its
// completed verdict as an "insufficient" proof, so its node walk is pruned
// nowhere: coveredSpendable keeps a restricted covered source there even
// where the plain census run would drop it. The proof run's gaps are
// per-node (visit's scriptNodeGaps), not the census's sr.gaps, so the fee
// loop's census enumeration can miss the fee the source's mana may pay:
// Isolinear Processor's {C} ("spend this mana only to cast an artifact spell
// or activate an ability of an artifact source") pays an artifact gap's
// activation fee that only a deeper node's census lists, never the sorcery
// play nor a land census gap's.
func TestPotentialScriptProofRunKeepsRestrictedFunding(t *testing.T) {
	t.Parallel()
	e := filterFeeBoard(t,
		[]string{"Isolinear Processor", "Swamp"},
		[]string{"Trickster's Stratagem"})
	strat := boundHandID(t, e, "Trickster's Stratagem")
	var proc, swamp state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		switch o := e.G.Obj(id); o.Face().Name {
		case "Isolinear Processor":
			proc = id
		case "Swamp":
			swamp = id
		}
	}
	if proc == 0 || swamp == 0 {
		t.Fatal("precondition: the Isolinear Processor or the Swamp is not on seat 0's battlefield")
	}
	if e.G.Obj(proc).Face().IsCreature() {
		t.Fatal("precondition: the source is a creature; the prune never applies")
	}
	sr := e.newScriptSearch(0, decision.PotentialAction{Kind: "cast", Obj: strat}, -1, 64)
	if sr == nil {
		t.Fatal("precondition: no pending priority decision for seat 0")
	}
	if sr.play.Kind != potentialCastKind {
		t.Fatalf("precondition: the searched play is %q, not a plain cast", sr.play.Kind)
	}
	// The plain census run prunes it: the restricted {C} is not spendable on
	// the sorcery cast, and the census gap's activation is no artifact
	// source's.
	sr.gaps = []state.ObjID{swamp}
	if sr.coveredSpendable(e, 0, proc) {
		t.Fatal("precondition: the plain census run keeps the restricted source; the proof run's guard is untested")
	}
	sr.nodePlans = true
	if !sr.coveredSpendable(e, 0, proc) {
		t.Fatal("the proof run pruned a restricted funding source its node walk must keep")
	}
}
