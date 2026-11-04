package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The "attached to a <class>" lockstep (kop.W7AttachedTo): Combadge makes
// an attacker unblockable while another creature wears a Communicator,
// Summoning Materia lets its controller cast creatures from the top of the
// library while it is attached, End Hostilities destroys every permanent
// attached to a creature, Miracle Worker destroys an Aura on its
// controller's creature and Sage's Reverie counts its controller's Auras on
// creatures. The expected answer at every decision is gorge's.
var kattachedToDecks = [2][]cardset.Entry{
	{
		{Name: "Combadge", Count: 4}, {Name: "Summoning Materia", Count: 2}, {Name: "Sage's Reverie", Count: 2},
		{Name: "Cartouche of Solidarity", Count: 3}, {Name: "Miracle Worker", Count: 2}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Hill Giant", Count: 2}, {Name: "Plains", Count: 9}, {Name: "Forest", Count: 5}, {Name: "Island", Count: 5},
	},
	{
		{Name: "Combadge", Count: 3}, {Name: "Bonesplitter", Count: 2}, {Name: "End Hostilities", Count: 2},
		{Name: "Miracle Worker", Count: 2}, {Name: "Cartouche of Solidarity", Count: 2}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Hill Giant", Count: 3}, {Name: "Shock", Count: 2}, {Name: "Plains", Count: 9}, {Name: "Mountain", Count: 9},
	},
}

func TestAttachedToLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "attachedto", kattachedToDecks, 400, "KATTACHEDTO_SEEDS")
}

// TestAttachedToLockstepMutation: with every W7AttachedTo read as "matches
// anything", some seed diverges.
func TestAttachedToLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "attachedto", kattachedToDecks, 200, false, func(tbl *kop.Table) {
		fs := append([]kop.Filter(nil), tbl.Filters...)
		for i := range fs {
			if fs[i].Pred == kop.PredWord7 && fs[i].PredN&0xff == kop.W7AttachedTo {
				fs[i].Pred, fs[i].PredN = kop.PredNone, 0
			}
		}
		tbl.Filters = fs
	})
	if n == 0 {
		t.Fatal("no seed diverged with the AttachedTo filters dropped")
	}
}
