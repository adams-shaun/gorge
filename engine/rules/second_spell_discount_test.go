package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// A "second spell you cast each turn costs {2} less" static discounts a
// sorcery cast after a resolved creature spell (CR 601.2f): the first spell is
// counted once it has been cast, and the spell being priced is not. A
// provenance-keyed reducer on the board ("spells cast from anywhere other than
// your hand cost {1} less") makes the cast re-price itself after the spell
// reaches the stack (CR 601.2a); that second pricing must not count the
// spell's own push as the earlier cast the gate wants.
func TestSecondSpellDiscountAfterResolvedCreature(t *testing.T) {
	t.Parallel()
	for _, withProvenance := range []bool{false, true} {
		secondSpellDiscountAfterResolvedCreature(t, withProvenance)
	}
}

func secondSpellDiscountAfterResolvedCreature(t *testing.T, withProvenance bool) {
	t.Helper()
	psionicSrc := "Name:Tidepool Savant\nManaCost:2 U\nTypes:Creature Jellyfish Scientist\nPT:2/4\n" +
		"S:Mode$ ReduceCost | ValidCard$ Card | Type$ Spell | Activator$ You | Amount$ 2 | CheckSVar$ SpellsCast | SVarCompare$ EQ1 | Description$ The second spell you cast each turn costs {2} less to cast.\n" +
		"SVar:SpellsCast:Count$ThisTurnCast_Card.YouCtrl\n" +
		"Oracle:The second spell you cast each turn costs {2} less to cast.\n"
	crabSrc := "Name:Reef Crab\nManaCost:U\nTypes:Creature Crab\nPT:0/3\nOracle:x\n"
	insightSrc := "Name:Moon Glance\nManaCost:2 U\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:Draw a card.\n"
	thiefSrc := "Name:Shore Thief\nManaCost:1 U\nTypes:Creature Rogue\nPT:2/2\n" +
		"S:Mode$ ReduceCost | Type$ Spell | Activator$ You | ValidCard$ Card.!wasCastFromYourHand | Amount$ 1 | Description$ A spell cast from outside the hand costs {1} less.\n" +
		"Oracle:Spells you cast from anywhere other than your hand cost {1} less to cast.\n"
	e, cfg, _ := newFixtureDeck(t, 531, insightSrc, crabSrc, psionicSrc, thiefSrc)
	putCreature(t, e, 0, psionicSrc)
	if withProvenance {
		putCreature(t, e, 0, thiefSrc)
	}
	toMain1(t, e)
	moveSeeded(t, e, 0, crabSrc, state.ZHand)
	insight := moveSeeded(t, e, 0, insightSrc, state.ZHand)
	addMana(t, e, 0, "U")
	opt := castByName(t, e, 0, "Reef Crab")
	if opt == nil {
		t.Fatal("Reef Crab should be castable with {U}")
	}
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(insight).Zone != state.ZHand {
		t.Fatalf("Moon Glance in %s, want hand", e.G.Obj(insight).Zone)
	}
	addMana(t, e, 0, "UBB")
	if got := reduceOf(t, e, 0, insight); got != 2 {
		t.Fatalf("second-spell reduction = %d, want 2", got)
	}
	opt = castByName(t, e, 0, "Moon Glance")
	if opt == nil {
		t.Fatal("Moon Glance should be castable")
	}
	submitChoices(t, e, opt.Index)
	for i := 0; i < 6 && e.G.Obj(insight).Zone == state.ZHand; i++ {
		submitChoices(t, e, 0)
	}
	var left int32
	for _, n := range e.G.Players[0].Pool {
		left += int32(n)
	}
	if left != 2 {
		t.Fatalf("provenance reducer %v: mana left %d after a {U} payment, want 2", withProvenance, left)
	}
	replayCheck(t, e, cfg)
}
