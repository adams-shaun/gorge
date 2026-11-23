package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Kinnan, Bonder Prodigy's "whenever you tap a nonland permanent for mana, add
// one mana of any type that permanent produced" (a reflected TapsForMana extra
// over ValidCard$ Permanent.nonLand, CR 106.12a) plays decision by decision
// against gorge: the extra prices on the tapped permanent's produced types, one
// mana of the produced type for a one-type source (a Sol Ring tap is three {C})
// and one more of the pick for a choice source (an Arcane Signet tap makes two
// of one colour). Both seats cast Kinnan first and otherwise pick at random.
var kinnanDecks = [2][]cardset.Entry{
	{
		{Name: "Kinnan, Bonder Prodigy", Count: 4}, {Name: "Sol Ring", Count: 4},
		{Name: "Mind Stone", Count: 4}, {Name: "Arcane Signet", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Opt", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Forest", Count: 7}, {Name: "Island", Count: 7},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Shock", Count: 8},
		{Name: "Sol Ring", Count: 2}, {Name: "Mountain", Count: 10},
		{Name: "Forest", Count: 10},
	},
}

// TestTapsForManaKinnanLockstep replays the pool's games: no divergence, the
// casts and activations both seats make.
func TestTapsForManaKinnanLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "tapsformana", kinnanDecks, 300, "TAPSMANA_KINNAN_SEEDS")
}

// TestTapsForManaKinnanMutation: with Kinnan's reflected tap-extra static
// dropped from the kernel's table, some seed diverges, so the seeds do exercise
// the extra (a green lockstep alone would pass a pool where nothing is ever
// tapped under it).
func TestTapsForManaKinnanMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "tapsformana", kinnanDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			var keep []kop.Ability
			for _, a := range cards[i].Abilities {
				if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatTapExtra &&
					a.Static.StFlags&kop.StTapReflect != 0 {
					continue
				}
				keep = append(keep, a)
			}
			cards[i].Abilities = keep
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with Kinnan's reflected tap-extra static dropped")
	}
}
