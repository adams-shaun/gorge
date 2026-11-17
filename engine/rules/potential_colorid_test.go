package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Cryptic Spires' "Produced$ Combo ColorID" (the circled colors) adds no mana:
// cards.ProducedCounts claims none for the word. The potential bound once
// priced it unbounded in every colour (the word's letters are no colour
// rune), so an untapped Spires made Eye of Vecna's "pay {2}" payable with
// every other source tapped (CR 118.3).
func TestPotentialManaColorIDAddsNothing(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	e.G.SetZone(state.ZHand, 0, nil)
	onBoard(t, e, 0, "Name:Circled Land\nTypes:Land\n"+
		"A:AB$ Mana | Cost$ T | Produced$ Combo ColorID | SpellDescription$ Add one mana of a circled colour.\nOracle:x\n")
	m := e.PotentialMana(0)
	for i, n := range m {
		if n != 0 {
			t.Fatalf("a ColorID source must add no potential mana, slot %d = %d (pool %v)", i, n, m)
		}
	}
}
