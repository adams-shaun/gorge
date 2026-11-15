package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Foil lockstep: "You may discard an Island card and another card rather
// than pay this spell's mana cost" (an AlternativeCost of Discard<1/Island>
// and Discard<1/Card.Other>, CR 118.9, 601.2h: gorge offers the
// AltCostIndex 1 cast from the hand while the hand pays both parts, then
// asks the counter's target, then one discard per part in order, the Island
// picked first gone and the cast Foil never offered). Island-heavy decks put
// the pitch in most opening hands. The expected answer at every decision is
// gorge's.
var kfoilDecks = [2][]cardset.Entry{
	{
		{Name: "Foil", Count: 8}, {Name: "Counterspell", Count: 4}, {Name: "Unsummon", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Island", Count: 18},
	},
	{
		{Name: "Foil", Count: 4}, {Name: "Shock", Count: 6}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Unsummon", Count: 4}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 10},
	},
}

// TestFoilAltCostLockstep plays the Foil pool in lockstep, then shows the pool
// plays the pitch cast and the kernel prices both parts: with the second part
// asking two cards (a hand of an Island and one other card no longer
// affords it), with the first part's Island filter dropped (any two cards
// pay), or with the alternative cost gone, some seed diverges.
func TestFoilAltCostLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "foil", kfoilDecks, 400, "KFOIL_SEEDS")
	for _, m := range []struct {
		name string
		fn   func(*kop.Ability)
	}{
		{"second part of two", func(a *kop.Ability) {
			if a.Cost.Discard2N != 1 || a.Cost.Flags&kop.CostDiscardFiltered == 0 {
				t.Fatalf("Foil's alternative cost is not an Island discard then one more card: %+v", a.Cost)
			}
			a.Cost.Discard2N = 2
		}},
		{"first part any card", func(a *kop.Ability) { a.Cost.Flags &^= kop.CostDiscardFiltered }},
		{"no alternative", func(a *kop.Ability) { a.Static.Kind = kop.StatNone }},
	} {
		n := lockstepRun(t, "foil", kfoilDecks, 200, false, func(tbl *kop.Table) {
			cards := append([]kop.CardDef(nil), tbl.Cards...)
			for i := range cards {
				abs := append([]kop.Ability(nil), cards[i].Abilities...)
				for j := range abs {
					if abs[j].Kind == kop.AbStatic && abs[j].Static.Kind == kop.StatAlternativeCost {
						m.fn(&abs[j])
					}
				}
				cards[i].Abilities = abs
			}
			tbl.Cards = cards
		})
		if n == 0 {
			t.Errorf("%s: no seed diverged: the pool never plays Foil's pitch cast, or the kernel does not price this part", m.name)
		}
	}
}
