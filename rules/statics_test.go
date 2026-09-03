package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestCantBeCastRemovesTheCastOption is row 1: CantBeCast | Caster$ Opponent
// gates by who is doing the casting, not by whose battlefield the restrictor
// sits on.
func TestCantBeCastRemovesTheCastOption(t *testing.T) {
	bear := card(t, "Name:Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	restrictor := card(t, "Name:Restrictor\nManaCost:1 W\nTypes:Artifact\n"+
		"S:Mode$ CantBeCast | ValidCard$ Creature | Caster$ Opponent\nOracle:x\n")

	e := handEngine(t, bear)
	e.G.Players[0].Pool[state.MG] = 1
	ro := e.G.AddObject(restrictor, 0)
	ro.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{ro.ID})

	if kinds(e.legalActions(0))["cast"] != 1 {
		t.Fatal("the restrictor's own controller should still be able to cast their creature")
	}

	// Give the opponent (seat 1) the same creature in hand, on their own turn.
	bear2 := e.G.AddObject(bear, 1)
	bear2.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 1, []state.ObjID{bear2.ID})
	e.G.Players[1].Pool[state.MG] = 1
	e.G.Active = 1

	if kinds(e.legalActions(1))["cast"] != 0 {
		t.Fatal("the opponent's creature cast should be restricted")
	}
}

// TestRaiseCostMakesASpellUnaffordable is row 2.
func TestRaiseCostMakesASpellUnaffordable(t *testing.T) {
	zap := card(t, "Name:Zap\nManaCost:1\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n")
	tax := card(t, "Name:Tax\nManaCost:1\nTypes:Artifact\n"+
		"S:Mode$ RaiseCost | ValidCard$ Instant | Amount$ 2\nOracle:x\n")
	e := handEngine(t, zap)
	to := e.G.AddObject(tax, 0)
	to.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{to.ID})

	e.G.Players[0].Pool[state.MC] = 1
	if kinds(e.legalActions(0))["cast"] != 0 {
		t.Fatal("a 1-mana spell raised by 2 should not be castable with 1 mana")
	}
	e.G.Players[0].Pool[state.MC] = 3
	if kinds(e.legalActions(0))["cast"] != 1 {
		t.Fatal("the raised cost of 3 should be castable with 3 mana")
	}
}

// TestReduceCostFloorsAtZero is row 3: CR 601.2f -- a reduction can eat the
// generic component but must never spill into the coloured requirement.
func TestReduceCostFloorsAtZero(t *testing.T) {
	zap := card(t, "Name:Zap\nManaCost:1 R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n")
	discount := card(t, "Name:Discount\nManaCost:1\nTypes:Artifact\n"+
		"S:Mode$ ReduceCost | ValidCard$ Instant | Amount$ 5\nOracle:x\n")
	e := handEngine(t, zap)
	do := e.G.AddObject(discount, 0)
	do.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{do.ID})

	id := e.G.Zone(state.ZHand, 0)[0]
	c := e.adjustedCost(0, id)
	if c.Generic != 0 {
		t.Fatalf("generic = %d, want 0 (floored, not negative)", c.Generic)
	}
	if c.Colored[state.MR] != 1 {
		t.Fatalf("the R requirement must survive the reduction untouched, got %v", c.Colored)
	}

	// End to end: only red mana in the pool, no generic at all.
	e.G.Players[0].Pool[state.MR] = 1
	if kinds(e.legalActions(0))["cast"] != 1 {
		t.Fatal("a cost reduced to bare {R} should be payable with a single red mana")
	}
}

// TestReduceCostAffectsActualPayment goes past option visibility (which the
// brief's own gating covers) to the mana actually spent when the option is
// submitted. This is not one of the brief's eight rows, but adjustedCost's
// own doc says it computes what a spell costs, and legalActions' gate is the
// only consumer the brief wires up; without also routing castSpell's payment
// through adjustedCost, a reduced-cost spell would still be charged its
// printed cost at resolution, which -- since payMana silently no-ops on an
// unpayable cost -- means the pool holding only the reduced amount would
// leave the spell cast for free instead of for the reduced price.
func TestReduceCostAffectsActualPayment(t *testing.T) {
	zap := card(t, "Name:Zap\nManaCost:1 R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n")
	discount := card(t, "Name:Discount\nManaCost:1\nTypes:Artifact\n"+
		"S:Mode$ ReduceCost | ValidCard$ Instant | Amount$ 1\nOracle:x\n")
	e := handEngine(t, zap)
	do := e.G.AddObject(discount, 0)
	do.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{do.ID})
	// Only the red pip's worth of mana: enough for the reduced {R} cost, not
	// enough for the printed {1}{R}.
	e.G.Players[0].Pool[state.MR] = 1
	e.askPriority(0)
	castFirst(t, e, "cast")

	if e.G.Players[0].Pool[state.MR] != 0 {
		t.Fatalf("pool = %v, want the R spent for the reduced cost", e.G.Players[0].Pool)
	}
	if len(e.G.Stack) != 1 {
		t.Fatal("the spell should have reached the stack")
	}
}

// TestRaiseCostAffectsActualPayment is the mirror check for the raise
// direction: undercharging (paying the printed cost instead of the raised
// one) is a quieter bug than ReduceCost's free-cast case, since payMana
// would still succeed, just for less than intended.
func TestRaiseCostAffectsActualPayment(t *testing.T) {
	zap := card(t, "Name:Zap\nManaCost:1\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n")
	tax := card(t, "Name:Tax\nManaCost:1\nTypes:Artifact\n"+
		"S:Mode$ RaiseCost | ValidCard$ Instant | Amount$ 2\nOracle:x\n")
	e := handEngine(t, zap)
	to := e.G.AddObject(tax, 0)
	to.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{to.ID})
	e.G.Players[0].Pool[state.MC] = 3
	e.askPriority(0)
	castFirst(t, e, "cast")

	if e.G.Players[0].Pool[state.MC] != 0 {
		t.Fatalf("pool = %v, want all 3 raised-cost mana spent", e.G.Players[0].Pool)
	}
}

// TestAlternativeCostAddsASecondCastOption is row 4.
func TestAlternativeCostAddsASecondCastOption(t *testing.T) {
	zap := card(t, "Name:Zap\nManaCost:2 R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\n"+
		"S:Mode$ AlternativeCost | ValidCard$ Card.Self | Cost$ 1 U\nOracle:x\n")
	e := handEngine(t, zap)
	e.G.Players[0].Pool[state.MR] = 1
	e.G.Players[0].Pool[state.MU] = 1
	e.G.Players[0].Pool[state.MC] = 2

	var labels []string
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" {
			labels = append(labels, o.Label)
		}
	}
	if len(labels) != 2 {
		t.Fatalf("expected two cast options (normal + alternative), got %v", labels)
	}
	if labels[0] == labels[1] {
		t.Fatalf("the two cast options must have distinct labels, got %v", labels)
	}
}

// TestCantBeActivatedSuppressesManaAbilities is row 5.
func TestCantBeActivatedSuppressesManaAbilities(t *testing.T) {
	mtn := card(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	hatepiece := card(t, "Name:Hatepiece\nManaCost:1 W\nTypes:Artifact\n"+
		"S:Mode$ CantBeActivated | ValidCard$ Land.OppCtrl\nOracle:x\n")

	e := handEngine(t)
	land := e.G.AddObject(mtn, 0)
	land.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{land.ID})

	hp := e.G.AddObject(hatepiece, 1)
	hp.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{hp.ID})

	if kinds(e.legalActions(0))["activate"] != 0 {
		t.Fatal("the land's mana ability should be suppressed by the opponent's static")
	}
}

// TestCantBlockRemovesEveryBlockOption is row 6. Block-option generation
// (askBlockers/handleBlockers) is still Task 21/22's stub, so this exercises
// blockRestricted directly rather than through a decision.
func TestCantBlockRemovesEveryBlockOption(t *testing.T) {
	blockerCard := card(t, "Name:Blocker\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\nOracle:x\n")
	attackerCard := card(t, "Name:Attacker\nManaCost:1 R\nTypes:Creature Goblin\nPT:2/2\nOracle:x\n")
	hex := card(t, "Name:Hex\nManaCost:1 U\nTypes:Enchantment\n"+
		"S:Mode$ CantBlock | ValidCard$ Creature\nOracle:x\n")

	e := handEngine(t)
	bl := e.G.AddObject(blockerCard, 0)
	bl.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{bl.ID})
	at := e.G.AddObject(attackerCard, 1)
	at.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{at.ID})

	if e.blockRestricted(bl.ID, at.ID) {
		t.Fatal("no restriction registered yet, blocking should be legal")
	}

	hx := e.G.AddObject(hex, 1)
	hx.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, append(e.G.Zone(state.ZBattlefield, 1), hx.ID))

	if !e.blockRestricted(bl.ID, at.ID) {
		t.Fatal("CantBlock should suppress every block for this creature")
	}
}

// TestCantBlockByRemovesOnlyMatchingPairs is row 7.
func TestCantBlockByRemovesOnlyMatchingPairs(t *testing.T) {
	smallCard := card(t, "Name:Small\nManaCost:W\nTypes:Creature Human\nPT:1/1\nOracle:x\n")
	bigCard := card(t, "Name:Big\nManaCost:2 W\nTypes:Creature Human\nPT:2/2\nOracle:x\n")
	attackerCard := card(t, "Name:Menacing\nManaCost:1 R\nTypes:Creature Goblin\nPT:2/2\n"+
		"S:Mode$ CantBlockBy | ValidCard$ Card.Self | ValidBlocker$ Creature.powerLE1\nOracle:x\n")

	e := handEngine(t)
	small := e.G.AddObject(smallCard, 0)
	small.Zone = state.ZBattlefield
	big := e.G.AddObject(bigCard, 0)
	big.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{small.ID, big.ID})
	att := e.G.AddObject(attackerCard, 1)
	att.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{att.ID})

	if !e.blockRestricted(small.ID, att.ID) {
		t.Fatal("the 1/1 should be restricted from blocking this attacker")
	}
	if e.blockRestricted(big.ID, att.ID) {
		t.Fatal("the 2/2 should still be able to block")
	}
}

// TestStaticsFromNonBattlefieldZonesDoNotApply is row 8.
func TestStaticsFromNonBattlefieldZonesDoNotApply(t *testing.T) {
	restrictor := card(t, "Name:Restrictor\nManaCost:1 W\nTypes:Artifact\n"+
		"S:Mode$ CantBeCast | ValidCard$ Creature\nOracle:x\n")
	bear := card(t, "Name:Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	e := handEngine(t, bear)
	ro := e.G.AddObject(restrictor, 0)
	ro.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{ro.ID})

	if e.castRestricted(0, e.G.Zone(state.ZHand, 0)[0]) {
		t.Fatal("a static on a card in the graveyard must not restrict anything")
	}
}

// TestActiveStaticsIsDeterministicallyOrdered guards the no-nondeterminism
// constraint: activeStatics must never let map iteration leak into its
// output order, since cost adjustment and the option list both depend on it
// being stable run to run.
func TestActiveStaticsIsDeterministicallyOrdered(t *testing.T) {
	a := card(t, "Name:A\nManaCost:1\nTypes:Artifact\nS:Mode$ RaiseCost | ValidCard$ Creature | Amount$ 1\nOracle:x\n")
	b := card(t, "Name:B\nManaCost:1\nTypes:Artifact\nS:Mode$ RaiseCost | ValidCard$ Creature | Amount$ 1\nOracle:x\n")

	e := handEngine(t)
	oa := e.G.AddObject(a, 0)
	oa.Zone = state.ZBattlefield
	ob := e.G.AddObject(b, 0)
	ob.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{oa.ID, ob.ID})

	var first []state.ObjID
	for i := 0; i < 20; i++ {
		var got []state.ObjID
		for _, sv := range e.activeStatics("RaiseCost") {
			got = append(got, sv.Source)
		}
		if i == 0 {
			first = got
			continue
		}
		if len(got) != len(first) {
			t.Fatalf("run %d: length changed: %v vs %v", i, got, first)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d: order changed at %d: %v vs %v", i, j, got, first)
			}
		}
	}
	if len(first) != 2 || first[0] != oa.ID || first[1] != ob.ID {
		t.Fatalf("expected [%d %d] in registration order, got %v", oa.ID, ob.ID, first)
	}
}
