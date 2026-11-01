package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The unearth lockstep (CR 702.84a): Yotian Frontliner's
// graveyard ability returns it with haste, exiles it at the next end step,
// and exiles it instead if it would leave the battlefield (gorge
// rules/unearth.go), against removal that kills, bounces and burns it. The
// expected answer at every decision is gorge's.
var kunearthDecks = [2][]cardset.Entry{
	{
		{Name: "Yotian Frontliner", Count: 12}, {Name: "Savannah Lions", Count: 10}, {Name: "Plains", Count: 18},
	},
	{
		{Name: "Murder", Count: 4}, {Name: "Unsummon", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Swamp", Count: 8}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 8},
	},
}

func TestUnearthLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "unearth", kunearthDecks, 1000, "KUNEARTH_SEEDS")
}

// TestUnearthLockstepMutation proves the pool unearths: with the riders
// dropped (a plain return), some seed diverges.
func TestUnearthLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "unearth", kunearthDecks, 200, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				p := tbl.Cards[i].Abilities[j].Prog
				for k := range p {
					if p[k].Op == kop.OpUnearth {
						p[k].Op = kop.OpNop
					}
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the unearth riders dropped: the pool never unearths")
	}
}

// TestUnearthLockstepLeaveMutation proves the pool meets the exile-instead
// promise: with it ignored (a killed or bounced unearthed creature goes to
// its graveyard or hand), some seed diverges.
func TestUnearthLockstepLeaveMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoUnearthLeave = true
	defer func() { dirac.DebugNoUnearthLeave = false }()
	if !lockstepDiverges(t, "unearth", kunearthDecks, 200) {
		t.Fatal("no seed diverged with the exile-instead promise ignored: no unearthed creature ever leaves early")
	}
}
