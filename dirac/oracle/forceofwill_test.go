package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Force of Will lockstep: "You may pay 1 life and exile a blue card from
// your hand rather than pay this spell's mana cost" (an AlternativeCost of
// PayLife<1> and ExileFromHand<1/Card.Blue+Other>, CR 118.9: gorge offers the
// AltCostIndex 1 cast from the hand, then the counter's target, then the
// pick of the blue card to exile), beside its {3}{U}{U} cast. Counterspell
// and Unsummon are the blue fodder, Force of Will itself the other blue
// card in hand. The expected answer at every decision is gorge's.
var kforceofwillDecks = [2][]cardset.Entry{
	{
		{Name: "Force of Will", Count: 8}, {Name: "Counterspell", Count: 4}, {Name: "Unsummon", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Island", Count: 18},
	},
	{
		{Name: "Force of Will", Count: 4}, {Name: "Shock", Count: 6}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Unsummon", Count: 4}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 10},
	},
}

func TestForceOfWillLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "forceofwill", kforceofwillDecks, 600, "KFORCEOFWILL_SEEDS")
}

// TestForceOfWillLockstepMutation: with the alternative cost dropped, or its
// shared count doubled (2 life and two blue cards, so a hand with one other
// blue card no longer affords it), some seed diverges, so the pool plays the
// pitch cast and the kernel prices its count.
func TestForceOfWillLockstepMutation(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name string
		fn   func(*kop.Ability)
	}{
		{"no alternative", func(a *kop.Ability) { a.Static.Kind = kop.StatNone }},
		{"two cards", func(a *kop.Ability) { a.Cost.N = 2 }},
	} {
		n := lockstepRun(t, "forceofwill", kforceofwillDecks, 200, false, func(tbl *kop.Table) {
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
			t.Errorf("%s: no seed diverged: the pool never plays Force of Will's pitch cast", m.name)
		}
	}
}
