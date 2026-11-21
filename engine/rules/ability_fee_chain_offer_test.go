package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const (
	feeChainMountain = "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"
	feeChainSwamp    = "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n"
)

// feeChainBackwoodsPlan puts Grim Backwoods ({2}{B}{G}, {T}, sacrifice a
// creature: draw), Flooded Grove ({G/U}, {T}: {G}{G}, {G}{U} or {U}{U}), a
// Mountain, two Swamps and a creature on p0's battlefield, plus an Island when
// funder, with the pool empty, and returns Backwoods' potential play (false:
// the potential walk does not list it). CR
// 605.3a / 602.2b: the Island's {U} pays Grove's fee, Grove's {G}{G} pays the
// {G} and one generic, a Swamp the {B}.
func feeChainBackwoodsPlan(t *testing.T, seed uint64, funder bool) (PotentialPlan, bool) {
	t.Helper()
	e, _, _ := newFixtureDeck(t, seed, ppFiller)
	e.paidManaCensus = true
	backwoods := onBoardCard(t, e, 0, corpusCard(t, "Grim Backwoods"))
	onBoardCard(t, e, 0, corpusCard(t, "Flooded Grove"))
	onBoardCard(t, e, 0, corpusCard(t, "Grizzly Bears"))
	onBoard(t, e, 0, feeChainMountain)
	onBoard(t, e, 0, feeChainSwamp)
	onBoard(t, e, 0, feeChainSwamp)
	if funder {
		onBoard(t, e, 0, lrIsland)
	}
	ppPriority(t, e)
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: pool %v not empty", e.G.Players[0].Pool)
	}
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == optAbility && state.ObjID(pp.Action.Obj) == backwoods {
			return pp, true
		}
	}
	return PotentialPlan{}, false
}

// TestAbilityFeeChainIsPricedByThePrefixScript: Grim Backwoods' draw is a
// priced potential play when the Island funds Flooded Grove's fee and Grove
// funds the cost. The planner census defers Grove (its fee), so the verdict is
// the prefix script's (potentialCensusFallback); this is the route the
// lockstep surface offers the play by (dirac/oracle gorge.go prScript), the
// kernel's actScriptPays being its mirror.
func TestAbilityFeeChainIsPricedByThePrefixScript(t *testing.T) {
	t.Parallel()
	pp, ok := feeChainBackwoodsPlan(t, 9701, true)
	if !ok {
		t.Fatal("Grim Backwoods' draw is not a potential play")
	}
	if pp.Reason != "" || len(pp.Script) == 0 {
		t.Fatalf("Island -> Flooded Grove -> cost: reason=%q detail=%q script=%d, want a priced script", pp.Reason, pp.Detail, len(pp.Script))
	}
}

// TestAbilityFeeChainWithoutTheFunderIsNotPriced: the same board without the
// Island has no {G}/{U} to pay Grove's fee, and three plain mana cannot pay
// {2}{B}{G}, so the play is
// either not listed or not priced.
func TestAbilityFeeChainWithoutTheFunderIsNotPriced(t *testing.T) {
	t.Parallel()
	pp, ok := feeChainBackwoodsPlan(t, 9702, false)
	if ok && (pp.Reason == "" || len(pp.Script) != 0) {
		t.Fatalf("no fee funder: reason=%q script=%d, want an unpriced play", pp.Reason, len(pp.Script))
	}
}
