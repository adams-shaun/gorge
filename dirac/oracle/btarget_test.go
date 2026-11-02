package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The becomes-target watcher lockstep: Pawpatch Recruit puts a
// +1/+1 counter on another creature its controller controls whenever an
// opponent's spell or ability targets one of theirs; Surrak, Elusive Hunter
// draws for the same, a creature spell on the stack included (Essence
// Scatter). The opponent targets with burn, a pinger and the counterspell;
// the watchers' own Giant Growth must not trigger them, and Tomakul Honor
// Guard's ward triggers beside them. The expected answer at every decision
// is gorge's.
var kbtargetDecks = [2][]cardset.Entry{
	{
		{Name: "Pawpatch Recruit", Count: 4}, {Name: "Surrak, Elusive Hunter", Count: 4}, {Name: "Tomakul Honor Guard", Count: 3},
		{Name: "Giant Growth", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 22},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Essence Scatter", Count: 4},
		{Name: "Prodigal Pyromancer", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Wind Drake", Count: 2},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

func TestBtargetLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "btarget", kbtargetDecks, 1000, "KBTARGET_SEEDS")
}

// TestBtargetLockstepMutation proves the pool exercises the watchers'
// source gate: with it opened to any controller's spell or ability, some
// seed diverges (the controller's own Giant Growth triggers them).
func TestBtargetLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "btarget", kbtargetDecks, 200, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				a := &tbl.Cards[i].Abilities[j]
				if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigBecomesTarget && a.Trig.Flags&kop.TfSelf == 0 {
					a.Trig.Whose = kop.WhoseAny
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the watchers' source gate open: the pool never targets its own watched creatures")
	}
}
