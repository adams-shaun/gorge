package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The EachDamage lockstep: Case of the Gateway Express's "each
// creature you control deals 1 damage to that creature" -- one damage batch
// whose hits each name their own source (gorge effEachDamage), lifelink
// (Healer's Hawk) gaining once per source -- and its solve on three
// attackers -- and Moonlight Hunt's Wolves dealing damage equal to their
// own power (Count$CardPower read per damager). The expected answer at every decision is gorge's.
var keachdmgDecks = [2][]cardset.Entry{
	{
		{Name: "Case of the Gateway Express", Count: 10}, {Name: "Savannah Lions", Count: 8}, {Name: "Healer's Hawk", Count: 6},
		{Name: "Plains", Count: 16},
	},
	{
		{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Timberpack Wolf", Count: 8},
		{Name: "Moonlight Hunt", Count: 6}, {Name: "Shock", Count: 2}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 12},
	},
}

func TestEachdmgLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "eachdmg", keachdmgDecks, 1000, "KEACHDMG_SEEDS")
}

// TestEachdmgLockstepMutation proves the pool deals the batch: with each
// damager's hit dropped, some seed diverges.
func TestEachdmgLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "eachdmg", keachdmgDecks, 200, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				p := tbl.Cards[i].Abilities[j].Prog
				for k := range p {
					if p[k].Ext() == kop.ExtEachDamage {
						p[k] = kop.Instr{Op: kop.OpNop}
					}
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the each-damage batch dropped")
	}
}
