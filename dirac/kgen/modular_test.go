package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Modular N (CR 702.43a) lowers from gorge's expansion: an entering-counters
// replacement and an optional dies trigger that puts the dying permanent's
// last-known +1/+1 count on a target artifact creature.
func TestModularLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{
		"Arcbound Bruiser", "Arcbound Condor", "Arcbound Crusher", "Arcbound Fiend", "Arcbound Hybrid",
		"Arcbound Javelineer", "Arcbound Lancer", "Arcbound Mouser", "Arcbound Prototype", "Arcbound Ravager",
		"Arcbound Reclaimer", "Arcbound Shikari", "Arcbound Slith", "Arcbound Stinger", "Arcbound Tracker",
		"Arcbound Wanderer", "Arcbound Whelp", "Arcbound Worker", "Power Depot", "Scrapyard Recombiner",
	} {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, lc.Refusals)
		}
	}
}
