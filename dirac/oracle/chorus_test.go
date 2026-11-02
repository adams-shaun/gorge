package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The chorus lockstep: Malcolm, Alluring Scoundrel's combat
// damage trigger puts a chorus counter on it, draws, discards (the card
// remembered), and with four or more chorus counters offers the discarded
// card's free cast from the graveyard (a Play node guarded by its
// ConditionPresent$). Eight Malcolms beside cheap spells to discard face an
// opponent of ground creatures and a little burn, so the flyer connects
// often enough to reach the fourth counter. The expected answer at every
// decision is gorge's.
var kchorusDecks = [2][]cardset.Entry{
	{
		{Name: "Malcolm, Alluring Scoundrel", Count: 10}, {Name: "Opt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Divination", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 5}, {Name: "Walking Corpse", Count: 6},
		{Name: "Unsummon", Count: 2}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Island", Count: 5},
	},
}

func TestChorusLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "chorus", kchorusDecks, 600, "KCHORUS_SEEDS")
}

// TestChorusLockstepMutation proves the pool reaches the fourth chorus
// counter: with the kernel's threshold raised to five, some seed diverges
// (gorge offers the free cast the kernel then withholds).
func TestChorusLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "chorus", kchorusDecks, 600, false, func(tbl *kop.Table) {
		for i := range tbl.Filters {
			f := &tbl.Filters[i]
			if f.Pred == kop.PredCountersGE && kop.CounterKind(f.PredN&0xff) == kop.CtrChorus {
				f.PredN = f.PredN&0xff | 5<<8
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the chorus threshold at five: the pool never reaches the fourth counter")
	}
}
