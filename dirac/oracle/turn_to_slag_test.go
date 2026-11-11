package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Turn to Slag: 5 damage to target creature, then "destroy all Equipment
// attached to that creature" (CR 608.2c: the instructions in order; the
// creature is still on the battlefield with its Equipment when the second
// one runs, SBAs wait for the spell to finish). Both decks carry Bonesplitter
// to be equipped (and to be destroyed, attached to the target and not when
// attached to another creature) beside creatures that survive or die to the
// damage. The expected answer at every decision is gorge's.
var kturnToSlagDecks = [2][]cardset.Entry{
	{
		{Name: "Turn to Slag", Count: 4}, {Name: "Bonesplitter", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Turn to Slag", Count: 2}, {Name: "Bonesplitter", Count: 6},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}

func TestTurnToSlagLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "turntoslag", kturnToSlagDecks, 200, "KTURNTOSLAG_SEEDS")
}

// TestTurnToSlagLockstepMutation: with the attached-to-target word dropped,
// the sweep destroys every Equipment, and some seed diverges (so the pool
// plays Turn to Slag against an equipped creature while another creature
// carries Equipment too).
func TestTurnToSlagLockstepMutation(t *testing.T) {
	t.Parallel()
	dropped := 0
	n := lockstepRun(t, "turntoslag", kturnToSlagDecks, 200, false, func(tbl *kop.Table) {
		fs := append([]kop.Filter(nil), tbl.Filters...)
		for i := range fs {
			if fs[i].Flags2&kop.F2AttachedToT0 != 0 {
				fs[i].Flags2 &^= kop.F2AttachedToT0
				dropped++
			}
		}
		tbl.Filters = fs
	})
	if dropped == 0 {
		t.Fatal("no filter carries F2AttachedToT0: the lowering is not the one under test")
	}
	if n == 0 {
		t.Fatal("no seed diverged with the attached-to-target word dropped")
	}
}
