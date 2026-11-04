package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The printed CantTarget lockstep (kop.StatCantTarget; CR 115.4): Species
// 8472, Thrun, Breaker of Silence, Fiendslayer Paladin, Shanna, Sisay's
// Legacy and Dense Foliage against artifact and non-artifact activated
// abilities, an artifact's triggered ability, and red, black and green
// spells. Each side also holds a source the other side's restrictions spare:
// its own Rod of Ruin, a green Giant Growth. The expected answer at every
// decision is gorge's.
var kcantTgtDecks = [2][]cardset.Entry{
	{
		{Name: "Species 8472", Count: 3}, {Name: "Thrun, Breaker of Silence", Count: 3}, {Name: "Fiendslayer Paladin", Count: 3},
		{Name: "Shanna, Sisay's Legacy", Count: 2}, {Name: "Dense Foliage", Count: 2}, {Name: "Rod of Ruin", Count: 2},
		{Name: "Giant Growth", Count: 2}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Forest", Count: 10}, {Name: "Plains", Count: 10},
	},
	{
		{Name: "Rod of Ruin", Count: 2}, {Name: "Fireforger's Puzzleknot", Count: 3}, {Name: "Pyrite Spellbomb", Count: 3},
		{Name: "Walking Ballista", Count: 2}, {Name: "Prodigal Sorcerer", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Doom Blade", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Mountain", Count: 9}, {Name: "Swamp", Count: 5}, {Name: "Forest", Count: 4},
	},
}

func TestCantTargetLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "canttgt", kcantTgtDecks, 400, "KCANTTGT_SEEDS")
}

// TestCantTargetLockstepMutation: with every printed CantTarget static
// dropped, a restricted permanent is offered as a target and some seed
// diverges.
func TestCantTargetLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "canttgt", kcantTgtDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbStatic && abs[j].Static.Kind == kop.StatCantTarget {
					abs[j].Static.Kind = kop.StatNone
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the CantTarget statics dropped")
	}
}
