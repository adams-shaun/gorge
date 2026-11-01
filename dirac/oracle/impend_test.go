package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The impending lockstep (CR 702.176a): Overlord of the
// Mistmoors cast for its impending cost enters with four time counters, is
// no creature until the last is removed at its controller's end steps
// (it neither attacks nor blocks), and is a 6/6 again after (gorge
// rules/impending.go), against a creature deck. The expected answer at
// every decision is gorge's. No targeted removal yet: gorge's
// effects/filter_type.go hasTypePrinted still reads a dormant permanent as
// a creature target (reported; the removal deck lands behind
// gorgePinAtLeast once the fix is pinned).
var kimpendDecks = [2][]cardset.Entry{
	{
		{Name: "Overlord of the Mistmoors", Count: 10}, {Name: "Savannah Lions", Count: 10}, {Name: "Plains", Count: 20},
	},
	{
		{Name: "Hill Giant", Count: 12}, {Name: "Savannah Lions", Count: 8}, {Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	},
}

func TestImpendLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "impend", kimpendDecks, 1000, "KIMPEND_SEEDS")
}

// A dormant Overlord has no creature types (CR 205.1a). Umbris, Fear Manifest
// puts Horror in the table's subtypes, so the projection compares it.
func TestImpendingDropsCreatureTypes(t *testing.T) {
	t.Parallel()
	deck := []cardset.Entry{
		{Name: "Overlord of the Mistmoors", Count: 10}, {Name: "Umbris, Fear Manifest", Count: 2},
		{Name: "Savannah Lions", Count: 8}, {Name: "Plains", Count: 20},
	}
	lockstepRun(t, "impend", [2][]cardset.Entry{deck, deck}, 60, true, nil)
}

// TestImpendLockstepTickMutation proves the pool ticks: with the end-step
// removal dropped, some seed diverges.
func TestImpendLockstepTickMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "impend", kimpendDecks, 200, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				p := tbl.Cards[i].Abilities[j].Prog
				for k := range p {
					if p[k].Op == kop.OpRemoveCounter && kop.CounterKind(p[k].C) == kop.CtrTime {
						p[k].Op = kop.OpNop
					}
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the time-counter removal dropped: the pool never impends")
	}
}

// TestImpendLockstepTypeMutation proves the pool meets the type switch:
// with an impending-dormant Overlord left a creature, some seed diverges.
func TestImpendLockstepTypeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoImpendTy = true
	defer func() { dirac.DebugNoImpendTy = false }()
	if !lockstepDiverges(t, "impend", kimpendDecks, 200) {
		t.Fatal("no seed diverged with the impending type switch ignored")
	}
}
