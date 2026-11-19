package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The swapped-line cut: a fee-only repeatable source's colour ask drops a
// colour a free bare-tap source can make, because the free source can hold
// the pip while the fee stays floating; the fee-only source can then make
// whatever colour the continuation needs at the same fee. The class board's
// Plains makes W for free and nothing makes B for free, so the cut drops the
// Refractor's W answer and the plain search reaches Orzhova's funding line in
// a handful of nodes; without the cut it walks the W subtree first and needs
// well over the smaller budget below.
func TestFeeOnlySwappedLineCutFindsTheLine(t *testing.T) {
	t.Parallel()
	e, orz := feeOnlyClassBoard(t)
	_, opts := e.potentialWalkOf(0, true)
	var refractor state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Energy Refractor" {
			refractor = id
		}
	}
	if refractor == 0 {
		t.Fatal("precondition: Energy Refractor is not on the battlefield")
	}
	play := -1
	for i, o := range opts {
		if o.Kind == "ability" && o.Obj == orz {
			play = i
		}
	}
	if play < 0 {
		t.Fatal("precondition: the potential walk has no Orzhova activation option")
	}
	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete || len(census.gaps) == 0 {
		t.Fatal("precondition: the census must leave the fee-only Refractor as a gap")
	}
	o := opts[play]
	a := decision.PotentialAction{Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode}
	desc, descOK := scriptPlayDesc(o, func() Cost { return e.castOfferBase(0, o.Obj) })
	if !descOK {
		t.Fatal("precondition: the play has a payment descriptor")
	}
	// The cut's read: a free bare tap makes W, and no free bare tap makes B.
	sr0 := e.newScriptSearch(0, a, o.AltCostIndex, potentialScriptNodes)
	if sr0 == nil {
		t.Fatal("precondition: no script search")
	}
	if !sr0.feeOnlyFreeTapColour(e, refractor, state.MW) {
		t.Fatal("precondition: the Plains is a free W source the cut must see")
	}
	if sr0.feeOnlyFreeTapColour(e, refractor, state.MB) {
		t.Fatal("precondition: no free bare-tap source makes B on the class board")
	}
	const budget = 10
	sr := e.newScriptSearch(0, a, o.AltCostIndex, budget)
	if sr == nil {
		t.Fatal("precondition: no script search")
	}
	sr.gaps, sr.gapsOnly, sr.funding, sr.printedOnly = census.gaps, true, census.fees(), true
	sr.feeOnly = true
	sr.actDesc, sr.actDescOK = desc, descOK
	sr.passes = true
	sr.feeStall = true
	script, found := sr.run(e)
	if !found || len(script) == 0 {
		t.Fatalf("the swapped line was not found within %d nodes (limited=%v nodes=%d)", budget, sr.limited, sr.nodes)
	}
	madeB := false
	for _, st := range script {
		for _, pick := range st.Picks {
			if pick.Obj == refractor && pick.Kind == "mana" && pick.ManaSymbol == "B" {
				madeB = true
			}
			if pick.Obj == refractor && pick.Kind == "mana" && pick.ManaSymbol == "W" {
				t.Fatalf("the cut kept the Refractor's dominated W answer: %v", script)
			}
		}
	}
	if !madeB {
		t.Fatalf("the found script does not make B through the Refractor: %v", script)
	}
}
