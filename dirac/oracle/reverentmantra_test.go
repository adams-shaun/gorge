package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Reverent Mantra lockstep: "You may exile a white card from your hand
// rather than pay this spell's mana cost" (an AlternativeCost static whose
// Cost$ is a bare ExileFromHand<1/Card.White+Other>, no life part, CR 118.9:
// gorge offers the AltCostIndex 1 cast from the hand, then the cast asks the
// chosen colour mid-resolution for ProtectionAll), beside its {3}{W} cast.
// Savannah Lions, Serra Angel and Silver Knight are the white hand fodder,
// Snapback the blue pitch twin (Unsummon its fodder) on the other seat. The
// expected answer at every decision is gorge's.
var kreverentmantraDecks = [2][]cardset.Entry{
	{
		{Name: "Reverent Mantra", Count: 8}, {Name: "Savannah Lions", Count: 6},
		{Name: "Serra Angel", Count: 4}, {Name: "Silver Knight", Count: 2},
		{Name: "Plains", Count: 20},
	},
	{
		{Name: "Snapback", Count: 4}, {Name: "Unsummon", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Shock", Count: 6}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 12},
	},
}

func TestReverentMantraLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "reverentmantra", kreverentmantraDecks, 600, "KREVERENTMANTRA_SEEDS")
}

// TestReverentMantraLockstepMutation: with the alternative cost dropped, some
// seed diverges, so the pool plays the exile cast and the kernel prices it.
func TestReverentMantraLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "reverentmantra", kreverentmantraDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbStatic && abs[j].Static.Kind == kop.StatAlternativeCost {
					abs[j].Static.Kind = kop.StatNone
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Errorf("no seed diverged: the pool never plays Reverent Mantra's exile cast")
	}
}
