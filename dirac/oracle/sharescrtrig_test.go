package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The "shares a creature type with the attacker" lockstep
// (kop.W7SharesCrTrigObj): Shared Animosity pumps each attacker by the other
// attackers that share a creature type with it, and Conjurer's Mantle's
// attack trigger looks for a library creature card sharing a type with the
// equipped attacker. Bears, Giants and changelings (every creature type)
// attack together. The expected answer at every decision is gorge's.
var ksharesCrTrigDecks = [2][]cardset.Entry{
	{
		{Name: "Shared Animosity", Count: 4}, {Name: "Grizzly Bears", Count: 7}, {Name: "Woodland Changeling", Count: 4},
		{Name: "Hill Giant", Count: 5}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Shared Animosity", Count: 3}, {Name: "Conjurer's Mantle", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 5}, {Name: "Woodland Changeling", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 7}, {Name: "Forest", Count: 6},
	},
}

func TestSharesCrTrigObjLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "sharescrtrig", ksharesCrTrigDecks, 400, "KSHARESCRTRIG_SEEDS")
}

// TestSharesCrTrigObjLockstepMutation: with every W7SharesCrTrigObj read as
// "matches anything", some seed diverges.
func TestSharesCrTrigObjLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "sharescrtrig", ksharesCrTrigDecks, 200, false, func(tbl *kop.Table) {
		fs := append([]kop.Filter(nil), tbl.Filters...)
		for i := range fs {
			if fs[i].Pred == kop.PredWord7 && fs[i].PredN&0xff == kop.W7SharesCrTrigObj {
				fs[i].Pred, fs[i].PredN = kop.PredNone, 0
			}
		}
		tbl.Filters = fs
	})
	if n == 0 {
		t.Fatal("no seed diverged with the shares-a-creature-type filters dropped")
	}
}
