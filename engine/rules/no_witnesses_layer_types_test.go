package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// shrineGolemFixture is an Enchantment whose own static makes it a 2/2
// Creature (a layer-4 type grant, as a solved Case does).
const shrineGolemFixture = "Name:Shrine Golem\nManaCost:2\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Card.Self | AddType$ Creature | SetPower$ 2 | SetToughness$ 2 | Description$ x\nOracle:x\n"

// TestNoWitnessesCountsLayerGrantedCreatures: "Each player who controls the
// most creatures investigates." A permanent's types are its characteristics
// after continuous effects (CR 613.1d, 205.1), so seat 0's animated enchantment
// counts: 2 creatures against 2 is a tie and both seats investigate. Counting
// the printed face gave 1 against 2 and seat 0 got no Clue.
func TestNoWitnessesCountsLayerGrantedCreatures(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, _ := qualifierEngine(t, reg, "No Witnesses")

	shrine := putToken(t, e, 0, shrineGolemFixture, state.ZBattlefield)
	putToken(t, e, 0, smallCreatureFixture, state.ZBattlefield)
	putToken(t, e, 1, smallCreatureFixture, state.ZBattlefield)
	putToken(t, e, 1, smallCreatureFixture, state.ZBattlefield)

	if !e.IsCreature(shrine) {
		t.Fatalf("precondition: the layer-4 static should make the enchantment a creature")
	}
	if o := e.G.Obj(shrine); o == nil || o.Zone != state.ZBattlefield || hasPrintedCreatureType(o) {
		t.Fatalf("precondition: shrine must be a battlefield permanent with no printed Creature type: %+v", o)
	}

	id := searchMoveByName(t, e, "No Witnesses", state.ZHand)
	addMana(t, e, 0, "WWWW")
	castAndResolve(t, e, id)

	if got := len(investigateClueIDs(t, e, 0)); got != 1 {
		t.Fatalf("seat 0 clues = %d, want 1 (2 creatures ties seat 1's 2)", got)
	}
	if got := len(investigateClueIDs(t, e, 1)); got != 1 {
		t.Fatalf("seat 1 clues = %d, want 1", got)
	}
}

func hasPrintedCreatureType(o *state.Object) bool {
	if f := o.Face(); f != nil {
		for _, ty := range f.Types {
			if ty == "Creature" {
				return true
			}
		}
	}
	return false
}
