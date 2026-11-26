package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// An until-end-of-turn Effect remembering its own source (RememberObjects$
// Self) whose trigger body reads it as Defined$ Remembered (kgen
// effectRememberSelf: the body's Remembered is the emblem's source object):
// Mountain Titan's counter on each black spell its controller casts.
var effRemSelfDecks = [2][]cardset.Entry{
	{
		{Name: "Mountain Titan", Count: 8}, {Name: "Walking Corpse", Count: 12},
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "Savannah Lions", Count: 16}, {Name: "Coral Merfolk", Count: 6},
		{Name: "Plains", Count: 10}, {Name: "Island", Count: 8},
	},
}

func TestEffectRememberSelfLockstep(t *testing.T) {
	t.Parallel()
	lockstepRun(t, "effremself", effRemSelfDecks, 80, true, nil)
}

// TestEffectRememberSelfLockstepMutation: with the emblems outliving the turn
// (UntilEOT dropped), some seed diverges: the games do reach the emblems.
func TestEffectRememberSelfLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "effremself", effRemSelfDecks, 80, false, func(tbl *kop.Table) {
		ems := append([]kop.Emblem(nil), tbl.Emblems...)
		for i := range ems {
			ems[i].UntilEOT = false
		}
		tbl.Emblems = ems
	})
	if n == 0 {
		t.Fatal("no seed diverged with the Self-remembering emblems outliving the turn")
	}
}
