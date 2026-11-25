package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// DelayedTrigger RememberObjects$ TriggeredTargetLKICopy on a per-referent
// DamageDone line: each Basilisk and Ohran Viper destroys a creature it dealt
// damage to at end of combat (the delayed program's remembered object is the
// trigger object, the damaged creature). Giant Spider and Grizzly Bears block
// and are blocked, so the damage lands on creatures both ways.
var dmgTgtDelayDecks = [2][]cardset.Entry{
	{
		{Name: "Lowland Basilisk", Count: 6}, {Name: "Ohran Viper", Count: 5}, {Name: "Stone-Tongue Basilisk", Count: 2},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Giant Spider", Count: 4}, {Name: "Forest", Count: 17},
	},
	{
		{Name: "Serpentine Basilisk", Count: 6}, {Name: "Lowland Basilisk", Count: 4}, {Name: "Ohran Viper", Count: 3},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Giant Spider", Count: 4}, {Name: "Forest", Count: 17},
	},
}

func TestDmgTargetDelayedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	lockstepRun(t, "dmgtgtdelay", dmgTgtDelayDecks, 200, true, nil)
}

// TestDmgTargetDelayedLockstepMutation: with the delayed destroy dropped from
// the Basilisks and Ohran Viper, some seed diverges.
func TestDmgTargetDelayedLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	n := lockstepRun(t, "dmgtgtdelay", dmgTgtDelayDecks, 200, false, func(tbl *kop.Table) {
		for _, name := range []string{"Lowland Basilisk", "Ohran Viper", "Serpentine Basilisk", "Stone-Tongue Basilisk"} {
			if mutateCard(t, tbl, name, func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpDelayedProgram {
					in.Op = kop.OpNop
				}
			}) == 0 {
				t.Fatalf("%s has no DelayedTrigger to drop", name)
			}
		}
	})
	if n == 0 {
		t.Error("no seed diverged with the delayed destroy dropped")
	}
}
