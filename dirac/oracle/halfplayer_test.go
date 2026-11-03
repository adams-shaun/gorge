package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The half-of-a-targeted-player lockstep: Rush of Dread is a spree sorcery
// whose modes make the targeted opponent sacrifice, discard and lose half
// of their creatures, hand and life, rounded up (Mode$ TgtChoose on the
// sacrifice, ThisTargetedPlayer$ counts with /HalfUp). The opponent keeps a
// board, a hand and a life total worth halving. The expected answer at every
// decision is gorge's.
var khalfDecks = [2][]cardset.Entry{
	{
		{Name: "Rush of Dread", Count: 8}, {Name: "Walking Corpse", Count: 14}, {Name: "Swamp", Count: 18},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 4},
		{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 9},
	},
}

func TestHalfPlayerLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "halfplayer", khalfDecks, 300, "KHALF_SEEDS")
}

// TestHalfPlayerLockstepMutation proves the pool casts the spree: with every
// Rush of Dread mode's amount cut to one, some seed diverges.
func TestHalfPlayerLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "halfplayer", khalfDecks, 300, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			if tbl.Cards[i].Name != "Rush of Dread" {
				continue
			}
			for j := range tbl.Cards[i].Abilities {
				for k := range tbl.Cards[i].Abilities[j].Prog {
					if in := &tbl.Cards[i].Abilities[j].Prog[k]; in.A.K != kop.VConst && (in.Op == kop.OpSacrifice || in.Op == kop.OpDiscard || in.Op == kop.OpLoseLife) {
						in.A = kop.Const(1)
					}
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the halved amounts cut to one: the pool never casts Rush of Dread's modes")
	}
}
