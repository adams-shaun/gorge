package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ControlledBy TriggeredDefendingPlayer / DefenderCtrl in a becomes-blocked
// or unblocked trigger's target filter (kgen lower.go defenderTrigBody):
// Goblin Vandal's and Orcish Squatters' unblocked triggers and Rust Scarab's
// blocked one target only what the defending player controls, the kernel's
// F2DefendingCtrl read. Both decks hold artifacts and lands on both sides, so
// a filter that ignored the role would offer the attacker's own permanents.
var defenderCtrlDecks = [2][]cardset.Entry{
	{
		{Name: "Goblin Vandal", Count: 8}, {Name: "Orcish Squatters", Count: 6}, {Name: "Rust Scarab", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Mountain", Count: 18},
	},
	{
		{Name: "Rust Scarab", Count: 6}, {Name: "Ornithopter", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Goblin Vandal", Count: 4},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 8},
	},
}

func TestControlledByDefendingPlayerLockstep(t *testing.T) {
	t.Parallel()
	lockstepRun(t, "defenderctrl", defenderCtrlDecks, 80, true, nil)
}

// TestControlledByDefendingPlayerLockstepMutation: with the defending-controller
// role dropped from every filter, some seed diverges (the kernel would offer
// the attacker's own artifacts and lands).
func TestControlledByDefendingPlayerLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "defenderctrl", defenderCtrlDecks, 80, false, func(tbl *kop.Table) {
		fs := append([]kop.Filter(nil), tbl.Filters...)
		for i := range fs {
			fs[i].Flags2 &^= kop.F2DefendingCtrl
		}
		tbl.Filters = fs
	})
	if n == 0 {
		t.Fatal("no seed diverged with the defending-controller role dropped")
	}
}
