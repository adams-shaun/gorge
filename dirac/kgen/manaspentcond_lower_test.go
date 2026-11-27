package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// manaSpentCondSpells read a mana-spent condition outside their own enters
// trigger (ConditionManaSpent$, CR 107.4) or a producer-tagged
// Count$CastTotalManaSpent (Treasure / Cave / Desert); each should lower with
// no refusal.
var manaSpentCondSpells = []string{
	"Boros Fury-Shield", "Dryad's Caress", "Firespout", "Flash Conscription", "Induce Paranoia",
	"Invert the Skies", "Mythos of Brokkos", "Repel Intruders", "Ribbons of Night", "River's Grasp",
	"Rolling Spoil", "Seed Spark", "Torrent of Souls", "Unnerving Assault",
	// producer-tagged Count$CastTotalManaSpent. Bat Colony is checked beside a Cave
	// land (TestManaSpentCondBatColony: its Cave.YouCtrl filter needs the
	// pool's Cave subtype). Cankerous Thirst is
	// omitted: its other blocker (a Pump OptionQuestion) is outside this slice.
	"Devour Intellect", "Hired Hexblade", "Jaded Sell-Sword", "Price of Loyalty",
	"Spoils of the Hunt", "Cataclysmic Prospecting", "Marut", "Prossh, Skyraider of Kher",
}

func TestManaSpentCondSlice(t *testing.T) {
	for _, name := range manaSpentCondSpells {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// TestManaSpentCondBatColony: Bat Colony's Cave tally read lowers in a pool
// that names the Cave subtype (Hidden Courtyard), as the census's pool does.
func TestManaSpentCondBatColony(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p := &pool{names: []string{"Bat Colony", "Hidden Courtyard"}, byName: map[string]*cards.Card{}, nDecks: map[string]int{}, reg: reg}
	for _, n := range p.names {
		c, ok := reg.Lookup(n)
		if !ok || c == nil {
			t.Fatalf("%s not in the corpus", n)
		}
		p.byName[n] = c
	}
	for _, c := range lowerPool(p).cards {
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", c.Name, c.Refusals)
		}
	}
}
