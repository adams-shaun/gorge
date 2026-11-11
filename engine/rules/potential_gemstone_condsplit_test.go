package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// condSplitVerdict poses seat 0's priority and reports the planner's verdict
// for casting spell: listed is whether the potential walk offers the cast at
// all (PotentialMana prices it), pp its PotentialPaymentPlans entry when
// listed, and proven whether the offer builder then lists it among its
// proven-unpayable casts.
func condSplitVerdict(t *testing.T, e *Engine, spell state.ObjID) (pp PotentialPlan, listed, proven bool) {
	t.Helper()
	if z := e.G.Obj(spell).Zone; z != state.ZHand {
		t.Fatalf("precondition: spell %d in %s, want hand", spell, z)
	}
	d := ppPriority(t, e)
	for _, c := range e.PotentialPaymentPlans(0) {
		if c.Action.Kind == "cast" && c.Action.Obj == spell {
			pp, listed = c, true
		}
	}
	e.EnsurePaymentActions()
	return pp, listed, containsObjID(d.PotentialUnpayableCasts, spell)
}

// condSplitPayable fails unless the cast is offered and not proven unpayable.
func condSplitPayable(t *testing.T, e *Engine, spell state.ObjID, what string) {
	t.Helper()
	pp, listed, proven := condSplitVerdict(t, e, spell)
	if !listed {
		t.Fatalf("%s: cast absent from the potential walk (PotentialMana %v), want offered", what, e.PotentialMana(0))
	}
	if proven || pp.Reason == "insufficient" {
		t.Errorf("%s: cast proven unpayable (in PotentialUnpayableCasts %v, verdict %+v), want payable (a prefix script is fine)", what, proven, pp)
	}
}

// condSplitUnpayable fails unless the cast is unpayable: absent from the
// potential walk, or listed and proven unpayable.
func condSplitUnpayable(t *testing.T, e *Engine, spell state.ObjID, what string) {
	t.Helper()
	pp, listed, proven := condSplitVerdict(t, e, spell)
	if listed && !proven {
		t.Errorf("%s: cast offered and not proven unpayable (verdict %+v), want unpayable", what, pp)
	}
}

// TestPotentialLuckyGemstoneFundsAColouredCast: Gemstone Caverns' mana ability
// is Condition-split (root {C} when it has no luck counter, sub "any colour"
// when it has one, CR 605.1a). With a luck counter it is the only source of a
// {U} instant, which is payable, so the planner must not prove it unpayable.
// Without the counter the same {U} spell IS unpayable: the two boards differ
// only by the counter, so the test cannot pass vacuously.
func TestPotentialLuckyGemstoneFundsAColouredCast(t *testing.T) {
	t.Parallel()
	const spellSrc = "Name:Coloured Probe\nManaCost:U\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

	// Lucky: payable.
	e, _, spell := newFixtureDeck(t, 9861, spellSrc)
	toMain1(t, e)
	caverns := onBoardCard(t, e, 0, corpusCard(t, "Gemstone Caverns"))
	e.emit(events.Event{Kind: events.CounterChange, Obj: caverns, Counter: "LUCK", Amount: 1})
	if n := e.G.Obj(caverns).Counter("LUCK"); n != 1 {
		t.Fatalf("precondition: Gemstone Caverns LUCK = %d, want 1", n)
	}
	condSplitPayable(t, e, spell, "lucky Gemstone Caverns, {U} spell")

	// Unlucky: unpayable (the root only adds {C}).
	e2, _, spell2 := newFixtureDeck(t, 9862, spellSrc)
	toMain1(t, e2)
	caverns2 := onBoardCard(t, e2, 0, corpusCard(t, "Gemstone Caverns"))
	if n := e2.G.Obj(caverns2).Counter("LUCK"); n != 0 {
		t.Fatalf("precondition: unlucky Gemstone Caverns LUCK = %d, want 0", n)
	}
	condSplitUnpayable(t, e2, spell2, "unlucky Gemstone Caverns, {U} spell")
}

// TestPotentialRiverOfTearsFundsABlackCast: River of Tears adds {U}, or {B}
// instead once its controller played a land this turn. After a land drop it
// is the only source of a {B} instant, which is payable; with no land played
// the same spell is unpayable.
func TestPotentialRiverOfTearsFundsABlackCast(t *testing.T) {
	t.Parallel()
	const spellSrc = "Name:Black Probe\nManaCost:B\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

	// A land played this turn: payable.
	e, _, spell := newFixtureDeck(t, 9863, spellSrc)
	toMain1(t, e)
	onBoardCard(t, e, 0, corpusCard(t, "River of Tears"))
	e.emit(events.Event{Kind: events.LandPlayed, Player: 0})
	if n := e.G.Players[0].LandsPlayed; n < 1 {
		t.Fatalf("precondition: LandsPlayed = %d, want >= 1", n)
	}
	condSplitPayable(t, e, spell, "River of Tears after a land drop, {B} spell")

	// No land played: unpayable (the root only adds {U}).
	e2, _, spell2 := newFixtureDeck(t, 9864, spellSrc)
	toMain1(t, e2)
	onBoardCard(t, e2, 0, corpusCard(t, "River of Tears"))
	if n := e2.G.Players[0].LandsPlayed; n != 0 {
		t.Fatalf("precondition: LandsPlayed = %d, want 0", n)
	}
	condSplitUnpayable(t, e2, spell2, "River of Tears with no land played, {B} spell")
}
