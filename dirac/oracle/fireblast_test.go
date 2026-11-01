package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Fireblast lockstep: "you may sacrifice two Mountains rather
// than pay this spell's mana cost" (a sacrifice-only AlternativeCost: gorge
// offers the AltCostIndex 1 cast from the hand, then the target, then the
// two-Mountain sacrifice pick), beside its {4}{R}{R} cast. The expected
// answer at every decision is gorge's.
var kfireblastDecks = [2][]cardset.Entry{
	{
		{Name: "Fireblast", Count: 12}, {Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 16}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Fireblast", Count: 6}, {Name: "Grizzly Bears", Count: 12}, {Name: "Mountain", Count: 12}, {Name: "Forest", Count: 10},
	},
}

func TestFireblastLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "fireblast", kfireblastDecks, 600, "KFIREBLAST_SEEDS")
}

// TestFireblastLockstepMutation: with the alternative cost dropped, some
// seed diverges.
func TestFireblastLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "fireblast", kfireblastDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbStatic && abs[j].Static.Kind == kop.StatAlternativeCost {
					abs[j].Cost.N = 0 // no longer the sacrifice alternative: prepAltFree skips it
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the alternative cost dropped")
	}
}
