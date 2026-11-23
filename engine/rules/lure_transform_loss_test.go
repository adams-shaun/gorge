package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 613.7a/613.7g + 613.1f: the lure a Continuous static's
// AddHiddenKeyword$ "All creatures able to block CARDNAME do so." granted an
// enchanted TOKEN is removed by a "creature tokens you control lose all
// abilities" static (Poppet Factory) whose permanent TRANSFORMED after the
// Aura attached -- a transform restamps the loss (CR 613.7g), so it is newer
// than the grant and layer 6's timestamp order removes the grant. A grant
// made after the transform survives it (the mirror of CR 613.7a's Rune of
// Flight example).
const lureTransformLossAura = "Name:Lure Fixture\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
	"S:Mode$ Continuous | AffectedDefined$ Enchanted | Affected$ Creature | AddHiddenKeyword$ All creatures able to block CARDNAME do so. | Description$ x\n" +
	"S:Mode$ Continuous | AffectedDefined$ Enchanted | AddPower$ 1 | AddToughness$ 1 | Description$ x\nOracle:x\n"

const lureTransformLossStitcher = "Name:Stitcher Fixture\nManaCost:2 U\nTypes:Creature Human Wizard\nPT:2/3\nAlternateMode:DoubleFaced\n" +
	"Oracle:x\nALTERNATE\nName:Factory Fixture\nManaCost:no cost\nColors:blue\nTypes:Artifact\n" +
	"S:Mode$ Continuous | Affected$ Creature.token+YouCtrl | SetPower$ 3 | SetToughness$ 3 | RemoveAllAbilities$ True | Description$ x\nOracle:x\n"

const lureTransformLossBear = "Name:Bear Fixture\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nK:Flying\nOracle:x\n"

func attachLureToToken(t *testing.T, e *Engine, token state.ObjID) state.ObjID {
	t.Helper()
	addMana(t, e, 0, "G")
	castFirst(t, e, "cast")
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	aura := e.G.Zone(state.ZBattlefield, 0)
	var attached state.ObjID
	for _, id := range aura {
		if e.G.Obj(id).AttachedTo == token {
			attached = id
		}
	}
	if attached == 0 {
		t.Fatalf("precondition: no Aura attached to the token %d", token)
	}
	return attached
}

func TestLureGrantDiesWithALaterTransformedLoss(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 64, lureTransformLossAura, lureTransformLossStitcher, lureTransformLossBear)
	bear := putToken(t, e, 0, lureTransformLossBear, state.ZBattlefield)
	stitcher := putCreature(t, e, 0, lureTransformLossStitcher)
	if combat.DerivedHiddenFlags(asBoard(e), bear).AllMustBlock {
		t.Fatal("precondition: the token is a lure before the Aura")
	}
	attachLureToToken(t, e, bear)
	if !combat.DerivedHiddenFlags(asBoard(e), bear).AllMustBlock {
		t.Fatal("precondition: the enchanted token is not a lure")
	}
	e.emit(events.Event{Kind: events.FlipFace, Obj: stitcher, Amount: 1, Text: "Transformed"})
	if e.G.Obj(stitcher).FaceIdx != 1 {
		t.Fatal("precondition: the Stitcher did not transform")
	}
	if asBoard(e).HasKW(bear, combat.KWFlying) {
		t.Fatal("precondition: the loss did not strip the token's printed keyword")
	}
	if combat.DerivedHiddenFlags(asBoard(e), bear).AllMustBlock {
		t.Fatal("the lure grant survived a loss restamped later than it (CR 613.7g)")
	}
}

// The mirror: a grant made after the transform is newer than the loss and
// survives it (CR 613.7a's Rune of Flight example).
func TestLureGrantSurvivesAnEarlierTransformedLoss(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 64, lureTransformLossAura, lureTransformLossStitcher, lureTransformLossBear)
	bear := putToken(t, e, 0, lureTransformLossBear, state.ZBattlefield)
	stitcher := putCreature(t, e, 0, lureTransformLossStitcher)
	e.emit(events.Event{Kind: events.FlipFace, Obj: stitcher, Amount: 1, Text: "Transformed"})
	if asBoard(e).HasKW(bear, combat.KWFlying) {
		t.Fatal("precondition: the loss did not strip the token's printed keyword")
	}
	if combat.DerivedHiddenFlags(asBoard(e), bear).AllMustBlock {
		t.Fatal("precondition: the unenchanted token is a lure")
	}
	attachLureToToken(t, e, bear)
	if !combat.DerivedHiddenFlags(asBoard(e), bear).AllMustBlock {
		t.Fatal("the lure grant died with an earlier loss (CR 613.7a)")
	}
}
