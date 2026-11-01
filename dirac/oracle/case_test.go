package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Case lockstep (CR 719.3b): Cases solved at their
// controller's end step -- Crimson Pulse with no cards in hand (Hellbent$),
// Filched Falcon with three artifacts (IsPresent2$), Gorgon's Kiss with
// three creature cards put into graveyards this turn -- and their "Solved"
// abilities (an upkeep rummage, a sacrifice activation, a 4/4 Gorgon
// static). The expected answer at every decision is gorge's.
var kcaseDecks = [2][]cardset.Entry{
	{
		{Name: "Case of the Crimson Pulse", Count: 8}, {Name: "Case of the Filched Falcon", Count: 8}, {Name: "Ornithopter", Count: 8},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Case of the Gorgon's Kiss", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Hill Giant", Count: 8},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 12},
	},
}

func TestCaseLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "case", kcaseDecks, 1000, "KCASE_SEEDS")
}

// TestCaseLockstepMutation proves the pool solves Cases: with the solve
// dropped, some seed diverges.
func TestCaseLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "case", kcaseDecks, 200, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				p := tbl.Cards[i].Abilities[j].Prog
				for k := range p {
					if p[k].Op == kop.OpSaddle && p[k].C == 1 {
						p[k].Op = kop.OpNop
					}
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the solve dropped: the pool never solves a Case")
	}
}
