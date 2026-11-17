package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 508.1 with Orgg's real corpus static: "Orgg can't attack if defending
// player controls an untapped creature with power 3 or greater" is a
// CantAttack line whose UnlessDefender$ property is
// !controlsCreature.untapped+powerGE3. The power predicate must read the
// candidate's LAYER-derived power: Ancient Ooze prints 0/0 and its
// characteristic-defining ability makes it 4/4 here (a 4-mana creature on the
// defender's battlefield). Reading the printed face called it 0, so the
// negation released Orgg and the attack was offered -- the lockstep
// divergence the Orgg recording pins. The layer-aware matcher closure in
// combat.AttackBlocked performs the derived read (origin/main's Mogg Jailer /
// Gaea's Might change), so this test pins that base behaviour directly.
func TestCantAttackUnlessDefenderPowerReadsCDA(t *testing.T) {
	t.Parallel()
	e := attackBlockedRegressionEngine(t)
	orgg := onBoardCard(t, e, 0, corpusCard(t, "Orgg"))
	ooze := onBoardCard(t, e, 1, corpusCard(t, "Ancient Ooze"))
	// A 1/1 with mana value 4: it raises the CDA power above 3 without being a
	// power-3+ creature itself, so tapping the Ooze is what releases Orgg.
	sage := onBoard(t, e, 1, "Name:Small Sage\nManaCost:2 G G\nTypes:Creature Human\nPT:1/1\nOracle:x\n")

	o := e.G.Obj(ooze)
	if o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: Ancient Ooze is not an untapped battlefield permanent: %+v", o)
	}
	if p := o.Face().Power(); p != 0 {
		t.Fatalf("precondition: Ancient Ooze printed power = %d, want the 0 CDA face", p)
	}
	if p := e.Derived(ooze).Power; p < 3 {
		t.Fatalf("precondition: Ancient Ooze derived power = %d, want >= 3 (from the 4-mana Small Sage)", p)
	}
	if !e.attackBlocked(orgg, 1, 0) {
		t.Fatal("Orgg attacked although the defender's untapped 4/4 Ancient Ooze satisfies the power-3+ UnlessDefender")
	}

	e.G.Obj(ooze).Tapped = true
	if e.G.Obj(sage).Tapped {
		t.Fatal("precondition: the 1/1 Small Sage must stay untapped (it is not a power-3+ creature)")
	}
	if e.attackBlocked(orgg, 1, 0) {
		t.Fatal("Orgg stayed blocked although the only power-3+ creature the defender controls is tapped")
	}
}
