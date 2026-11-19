package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A fixed-colour fee-only gap ("{R}: Add {B}") beside a play that needs no
// {B} is as doomed as an any-colour one: the kernel's activationStalls prunes
// it by the same per-colour test, so feeOnlyGapDoomed must not fail open on a
// non-open production (the two searches walk the same space).
func TestFeeOnlyGapDoomedPrunesAFixedColourProduction(t *testing.T) {
	t.Parallel()
	e := boundBoard(t, []string{"Mountain", "Plains", "Agent of Stromgald"}, []string{"Savannah Lions"})
	_, opts := e.potentialWalkOf(0, true)
	play := -1
	for i, o := range opts {
		if o.Kind == "cast" && e.G.Obj(o.Obj) != nil && e.G.Obj(o.Obj).Face().Name == "Savannah Lions" {
			play = i
		}
	}
	if play < 0 {
		t.Fatal("precondition: the potential walk has no Savannah Lions cast option")
	}
	var agent state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face().Name == "Agent of Stromgald" {
			agent = id
		}
	}
	o := opts[play]
	a := decision.PotentialAction{Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode}
	sr := e.newScriptSearch(0, a, o.AltCostIndex, potentialScriptNodes)
	if sr == nil || !sr.pipOK {
		t.Fatal("precondition: the search reads the play's pips")
	}
	sr.actDesc, sr.actDescOK = scriptPlayDesc(o, func() Cost { return e.castOfferBase(0, o.Obj) })
	if !sr.actDescOK {
		t.Fatal("precondition: the play has a payment descriptor")
	}
	e.G.Players[0].Pool[state.MR] = 1 // the fee floats: the activation is payable
	if !sr.feeOnlyGapDoomed(e, agent) {
		t.Fatal("Agent of Stromgald's {R}: Add {B} fills no pip of {W}: the activation is doomed")
	}
}
