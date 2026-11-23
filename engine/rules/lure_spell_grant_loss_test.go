package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 509.1c / 613.11: a spell's unquoted "all creatures able to block target
// creature this turn do so" (Taunting Challenge's Pump KW$ HIDDEN grant,
// registered as a LAbilities effect on the target) is a requirement on the
// blockers, not an ability of the creature, so a layer-6 loss with a LATER
// timestamp cannot end it. The quoted static grant of a lure Aura is the
// opposite case (TestLureGrantDiesWithALaterTransformedLoss).
func TestLureSpellGrantSurvivesALaterTransformedLoss(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 64, lureTransformLossAura, lureTransformLossStitcher, lureTransformLossBear)
	bear := putToken(t, e, 0, lureTransformLossBear, state.ZBattlefield)
	stitcher := putCreature(t, e, 0, lureTransformLossStitcher)
	e.AddContinuous(state.ContinuousEffect{
		Source: bear, Affects: "Card.Self", Layer: state.LAbilities, UntilEOT: true,
		AddKeywords: []string{"All creatures able to block CARDNAME do so."},
	})
	if !combat.DerivedHiddenFlags(asBoard(e), bear).AllMustBlock {
		t.Fatal("precondition: the spell grant did not make the token a lure")
	}
	e.emit(events.Event{Kind: events.FlipFace, Obj: stitcher, Amount: 1, Text: "Transformed"})
	if e.G.Obj(stitcher).FaceIdx != 1 {
		t.Fatal("precondition: the Stitcher did not transform")
	}
	if asBoard(e).HasKW(bear, combat.KWFlying) {
		t.Fatal("precondition: the later loss did not strip the token's printed keyword")
	}
	if !combat.DerivedHiddenFlags(asBoard(e), bear).AllMustBlock {
		t.Fatal("a later ability loss ended a spell's lure requirement (CR 613.11)")
	}
}
