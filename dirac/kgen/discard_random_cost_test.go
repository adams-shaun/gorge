package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRandomDiscardCostLowers: Discard<N/Random> as an activated ability's or
// a spell's cost lowers to CostDiscard beside the DiscardRandom table byte
// (the engine RNG picks the cards, nothing is asked) for every corpus card
// whose only blocker it was.
func TestRandomDiscardCostLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name string
		n    uint8
	}{
		{"Acceptable Losses", 1}, {"Amok", 1}, {"Canyon Drake", 1}, {"Coral Helm", 1},
		{"Draconian Cylix", 1}, {"Dwarven Strike Force", 1}, {"Frenetic Ogre", 1}, {"Mage il-Vec", 1},
		{"Meteor Storm", 2}, {"Ogre Shaman", 1}, {"Pardic Lancer", 1}, {"Pardic Swordsmith", 1},
		{"Pyromania", 1}, {"Sonic Burst", 1}, {"Sonic Seizure", 1}, {"Stormbind", 1},
	} {
		card, ok := reg.Lookup(tc.name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", tc.name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		for _, rf := range lc.Refusals {
			t.Errorf("%s refused: %s", tc.name, rf.Reason)
		}
		_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
		if err != nil {
			t.Fatalf("%s build: %v", tc.name, err)
		}
		def := all[0].def
		saw := 0
		costs := []kop.Cost{def.SpellCost}
		for i := range def.Abilities {
			costs = append(costs, def.Abilities[i].Cost)
		}
		for _, c := range costs {
			if !c.DiscardRandom {
				continue
			}
			saw++
			if c.Flags&kop.CostDiscard == 0 || c.N != tc.n || c.DiscardHand {
				t.Errorf("%s: random discard cost flags %b N %d hand %v, want CostDiscard N %d", tc.name, c.Flags, c.N, c.DiscardHand, tc.n)
			}
		}
		if saw != 1 {
			t.Errorf("%s: %d costs carry DiscardRandom, want 1", tc.name, saw)
		}
	}
}
