package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The targeted EachDamage lockstep: DefinedDamagers$ ParentTarget
// (Common Goal, Coordinated Clobbering, Tandem Takedown, Terrific Team-Up,
// Band Together, Allies at Last: the parent link's one or two targets each
// deal damage equal to their power, after the link's pump), one damage
// batch with Healer's Hawk's lifelink, and gorge's TargetUnique$ chain
// look-ahead (Band Together, Tandem Takedown with one creature out). The
// expected answer at every decision is gorge's.
var keachdmg2Decks = [2][]cardset.Entry{
	{
		{Name: "Common Goal", Count: 3}, {Name: "Coordinated Clobbering", Count: 3}, {Name: "Tandem Takedown", Count: 3},
		{Name: "Terrific Team-Up", Count: 2}, {Name: "Band Together", Count: 2}, {Name: "Allies at Last", Count: 2},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Timberpack Wolf", Count: 4}, {Name: "Healer's Hawk", Count: 3},
		{Name: "Plains", Count: 3}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Jasmine Boreal", Count: 4}, {Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Shock", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 5},
	},
}

func TestEachDamageTargetedLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "eachdmgtargeted", keachdmg2Decks, 1000, "KEACHDMG2_SEEDS")
}

// TestEachDamageTargetedLockstepMutation proves the pool reads the damager slots:
// with the targeted each-damage dropped, some seed diverges.
func TestEachDamageTargetedLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "eachdmgtargeted", keachdmg2Decks, 300, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				p := tbl.Cards[i].Abilities[j].Prog
				for k := range p {
					if p[k].Ext() == kop.ExtEachDamage && p[k].C>>8 != 0 {
						p[k] = kop.Instr{Op: kop.OpNop}
					}
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the targeted each-damage dropped")
	}
}
