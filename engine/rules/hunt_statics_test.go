package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// hunt_statics_test.go pins the three Commander statics hunt-cmdr-statics adds:
// CantPayLife (Angel of Jubilation), CanAttackIfHaste (Instill Energy) and
// AttackRequirement (War's Toll).

func huntTwoSeat(t *testing.T) *Engine {
	t.Helper()
	return New(Config{Seed: 5, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
}

// TestCantPayLifeBlocksCostLife: the real Angel of Jubilation forbids either
// seat a Spell/Activated life payment; a payment of any other kind (ward,
// upkeep, unless) still pays; a game with no Angel pays (the control).
func TestCantPayLifeBlocksCostLife(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	angel := mustCorpusCard(t, reg, "Angel of Jubilation")
	greed := mustCorpusCard(t, reg, "Greed")
	if !effects.Supported()["stat:CantPayLife"] {
		t.Fatal("precondition: stat:CantPayLife is not a registered primitive")
	}
	e := huntTwoSeat(t)
	onBoardCard(t, e, 0, angel)
	greedID := onBoardCard(t, e, 1, greed)
	if len(e.activeStatics("CantPayLife")) == 0 {
		t.Fatal("precondition: the CantPayLife static is not active")
	}
	ctl := huntTwoSeat(t)
	ctlGreed := onBoardCard(t, ctl, 1, greed)
	if len(ctl.activeStatics("CantPayLife")) != 0 {
		t.Fatal("control precondition: the control game has a CantPayLife static")
	}
	for _, kind := range []pay.SAKind{pay.SASpell, pay.SAActivated} {
		for _, p := range []state.PlayerID{0, 1} {
			if !playerLifePayForbidden(e, p, kind) {
				t.Fatalf("seat %d kind %d: life payment not forbidden under Angel", p, kind)
			}
		}
	}
	lifeCost := pay.Cost{Life: 2}
	pool := func(g *Engine) (state.Mana, [7]state.Mana) {
		return g.G.Players[1].Pool, g.G.Players[1].ManaUnits()
	}
	p1, t1 := pool(e)
	if pay.CostPayablePool(asPayer(e), 1, greedID, pay.SAActivated, lifeCost, p1, t1) {
		t.Fatal("CostPayablePool priced a Greed-style 2-life activation under Angel")
	}
	if pay.CostPayablePool(asPayer(e), 1, greedID, pay.SASpell, lifeCost, p1, t1) {
		t.Fatal("CostPayablePool priced a 2-life spell cost under Angel")
	}
	if pay.CostPayableClass(asPayer(e), 1, pay.DescriptorFor(greedID, true, lifeCost), pay.PipRider{}, lifeCost) {
		t.Fatal("CostPayableClass priced a Greed-style 2-life activation under Angel")
	}
	if !pay.CostPayablePool(asPayer(e), 1, greedID, pay.SAOther, lifeCost, p1, t1) {
		t.Fatal("CostPayablePool refused a non-Spell/Activated 2-life payment under Angel")
	}
	if !pay.CostPayablePool(asPayer(e), 1, greedID, pay.SAActivated, pay.Cost{}, p1, t1) {
		t.Fatal("a zero-life activation must stay payable (CR 119.4b)")
	}
	c1, ct1 := pool(ctl)
	if !pay.CostPayablePool(asPayer(ctl), 1, ctlGreed, pay.SASpell, lifeCost, c1, ct1) ||
		!pay.CostPayablePool(asPayer(ctl), 1, ctlGreed, pay.SAActivated, lifeCost, c1, ct1) {
		t.Fatal("control: the 2-life payment is refused with no Angel on the board")
	}
	if !pay.CostPayableClass(asPayer(ctl), 1, pay.DescriptorFor(ctlGreed, true, lifeCost), pay.PipRider{}, lifeCost) {
		t.Fatal("control: CostPayableClass refused the 2-life activation with no Angel")
	}
}

// TestCantPayLifeCauseScoping: the kernel read on the Angel/Yasharn shape.
func TestCantPayLifeCauseScoping(t *testing.T) {
	e := huntTwoSeat(t)
	onBoardCard(t, e, 0, card(t, "Name:Gate Cant Pay\nTypes:Enchantment\n"+
		"S:Mode$ CantPayLife | ValidPlayer$ Player | ValidCause$ Spell,Activated | ForCost$ True\nOracle:x\n"))
	if len(e.activeStatics("CantPayLife")) == 0 {
		t.Fatal("precondition: the fixture static is not active")
	}
	if !playerLifePayForbidden(e, 0, pay.SASpell) || !playerLifePayForbidden(e, 0, pay.SAActivated) {
		t.Fatal("Spell/Activated life payment not forbidden")
	}
	if playerLifePayForbidden(e, 0, pay.SAOther) {
		t.Fatal("a ward/upkeep/unless payment (SAOther) must stay payable")
	}
	// ForCost$ False is the effect path: inert on the cost path.
	e2 := huntTwoSeat(t)
	onBoardCard(t, e2, 0, card(t, "Name:Gate Effect Only\nTypes:Enchantment\n"+
		"S:Mode$ CantPayLife | ValidPlayer$ Player | ValidCause$ Spell,Activated | ForCost$ False\nOracle:x\n"))
	if len(e2.activeStatics("CantPayLife")) == 0 || playerLifePayForbidden(e2, 0, pay.SASpell) {
		t.Fatal("a ForCost$ False line must be inert on the cost path")
	}
}
