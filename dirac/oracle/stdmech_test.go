package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The MageZero Standard mechanisms lockstep:
//   - Burnout Bashtronaut, Gastal Thrillseeker, Gastal Raider, Walking
//     Sarcophagus: Start your engines! and the max-speed granted static
//     (AddStaticAbility$: a keyword or P/T grant to itself at speed 4).
//
// The opponent fields Walls, a flier and bears to block the menace attackers. The expected
// answer at every decision is gorge's.
var kstdmechDecks = [2][]cardset.Entry{
	{
		{Name: "Burnout Bashtronaut", Count: 8}, {Name: "Gastal Thrillseeker", Count: 4}, {Name: "Gastal Raider", Count: 4},
		{Name: "Walking Sarcophagus", Count: 4}, {Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Mountain", Count: 9}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Wall of Stone", Count: 4}, {Name: "Wall of Wood", Count: 4}, {Name: "Wall of Swords", Count: 4},
		{Name: "Serra Angel", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Giant Growth", Count: 3},
		{Name: "Forest", Count: 9}, {Name: "Plains", Count: 9},
	},
}

func TestStdmechLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "stdmech", kstdmechDecks, 1000, "KSTDMECH_SEEDS")
}

// TestStdmechLockstepMutation proves the pool reaches max speed: with the
// max-speed double strike grant dropped, some seed diverges.
func TestStdmechLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "stdmech", kstdmechDecks, 200, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				a := &tbl.Cards[i].Abilities[j]
				if a.Kind == kop.AbStatic && a.Static.Cond2 != 0 && a.Static.Kw&kop.KwDoubleStrike != 0 {
					a.Static.Kw &^= kop.KwDoubleStrike
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the max-speed double strike dropped: the pool never reaches max speed")
	}
}
