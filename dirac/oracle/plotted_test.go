package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The BecomesPlotted lockstep: Aloe Alchemist's "when this card
// becomes plotted, target creature gets +3/+2 and gains trample until end of
// turn" (gorge trigmatch/plotted.go BecomesPlottedMatches), plotted for
// {1}{G} beside bears, against burn, a bigger creature and Longhorn
// Sharpshooter (its plot trigger deals 2 damage to any target). The expected
// answer at every decision is gorge's.
var kplottedDecks = [2][]cardset.Entry{
	{
		{Name: "Aloe Alchemist", Count: 12}, {Name: "Grizzly Bears", Count: 8}, {Name: "Giant Growth", Count: 3},
		{Name: "Forest", Count: 17},
	},
	{
		{Name: "Hill Giant", Count: 4}, {Name: "Longhorn Sharpshooter", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}

func TestPlottedLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "plotted", kplottedDecks, 1000, "KPLOTTED_SEEDS")
}

// TestPlottedLockstepMutation proves the pool plots Aloe Alchemist: with
// its trigger's event changed (it never fires), some seed diverges.
func TestPlottedLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "plotted", kplottedDecks, 200, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				if a := &tbl.Cards[i].Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigBecomesPlotted {
					a.Trig.Event = kop.TrigUnlockDoor
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the BecomesPlotted trigger dropped: the pool never plots Aloe Alchemist")
	}
}
