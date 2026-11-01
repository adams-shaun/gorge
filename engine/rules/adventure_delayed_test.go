package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestAdventureDelayedTriggerOutlivesItsFace pins CR 603.7: Swift Spiral's
// return is promised by the Adventure face, and the card is back to its
// creature face in exile by the end step that fires it.
func TestAdventureDelayedTriggerOutlivesItsFace(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Twining Twins"))
	quarry := onBoard(t, e, 1, "Name:Quarry\nTypes:Creature Bear\nPT:4/4\nOracle:x\n")
	e.G.Players[0].Pool[state.MW], e.G.Players[0].Pool[state.MC] = 1, 1
	e.priorityRound()
	submitChoices(t, e, castModeOption(t, e, e.G.Zone(state.ZHand, 0)[0], "adventure_alt"))
	submitChoices(t, e, optionForObj(e.Pending(), quarry))
	passUntilStackEmpty(t, e, 20)
	if z := e.G.Obj(quarry).Zone; z != state.ZExile {
		t.Fatalf("Swift Spiral left its target in zone %v, want exile", z)
	}
	for i := 0; i < 40 && e.G.Step != state.StepEnd; i++ {
		passOnceP(t, e)
	}
	passUntilStackEmpty(t, e, 20)
	if z := e.G.Obj(quarry).Zone; z != state.ZBattlefield {
		t.Fatalf("the exiled creature is in zone %v at the end step, want the battlefield", z)
	}
}
