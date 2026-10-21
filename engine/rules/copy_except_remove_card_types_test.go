package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDinoDNACopyStripsCardTypesOnTheCopiableFace pins the RemoveCardTypes$
// strip on a real CopyPermanent (CR 707.9b): Dino DNA's token copy keeps only
// the copied card's supertypes before AddTypes$ makes it a Creature Dinosaur,
// so the printed creature subtype and card type are gone from the token's
// copiable face -- and therefore from a copy of that token too.
func TestDinoDNACopyStripsCardTypesOnTheCopiableFace(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Dino DNA"), lookup(t, reg, "Grizzly Bears")},
		[]*cards.Card{})
	dino := moveByName(t, e, 0, "Dino DNA", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZGraveyard)

	addMana(t, e, 0, "CCCCCCC") // {1} to imprint, then {6} to copy

	submitChoices(t, e, abilityOption(t, e, dino, 0).Index)
	answerKTarget(t, e, bear)
	passUntilStackEmpty(t, e, 20)

	submitChoices(t, e, abilityOption(t, e, dino, 1).Index)
	answerKTarget(t, e, bear)
	passUntilStackEmpty(t, e, 20)

	bearCard := e.G.Obj(bear).Card
	cid := findTokenCopyOf(t, e, bearCard)
	d := e.Derived(cid)
	if !copyHasTypeWord(d.Types, "Creature") || !copyHasTypeWord(d.Types, "Dinosaur") {
		t.Fatalf("Dino DNA copy types = %v, want Creature Dinosaur", d.Types)
	}
	if copyHasTypeWord(d.Types, "Bear") {
		t.Fatalf("Dino DNA copy types = %v still carry the printed Bear subtype; RemoveCardTypes$ must strip it", d.Types)
	}
	if d.Power != 6 || d.Toughness != 6 {
		t.Fatalf("Dino DNA copy P/T = %d/%d, want 6/6", d.Power, d.Toughness)
	}
	noCopyPermanentModNote(t, e, "RemoveCardTypes", "RemoveSubTypes", "AddTypes", "SetPower", "SetToughness")
	replayCheck(t, e, cfg)
}
