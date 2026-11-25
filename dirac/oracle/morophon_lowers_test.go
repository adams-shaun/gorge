package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Morophon, the Boundless's colour-only reduction in lockstep against gorge
// (CR 601.2f): spells of the chosen type cost {W}{U}{B}{R}{G} less (Edgewalker's
// Clerics {W}{B}, Ragemonger's Minotaurs {B}{R}), and the shortfall never
// spills to generic (IgnoreGeneric$ True; gorge pay
// CostMods.ApplyTo, kernel kop.ReduceColorNoSpill). The changelings are every
// creature type (Clerics and Minotaurs too), so whichever type is chosen prices them: Woodland
// Changeling's {1}{G} costs {1}, not {0}. The expected answer at every
// decision is gorge's.
const morophonPool = "morophon"

var morophonDecks = [2][]cardset.Entry{
	{
		{Name: "Morophon, the Boundless", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Elvish Mystic", Count: 4},
		{Name: "Mind Stone", Count: 2}, {Name: "Woodland Changeling", Count: 4}, {Name: "Fire-Belly Changeling", Count: 3},
		{Name: "Avian Changeling", Count: 3}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 3}, {Name: "Plains", Count: 3},
	},
	{
		{Name: "Morophon, the Boundless", Count: 2}, {Name: "Llanowar Elves", Count: 4}, {Name: "Mind Stone", Count: 3},
		{Name: "Edgewalker", Count: 3}, {Name: "Ragemonger", Count: 3}, {Name: "Changeling Outcast", Count: 4},
		{Name: "Taurean Mauler", Count: 2}, {Name: "Woodland Changeling", Count: 3},
		{Name: "Forest", Count: 5}, {Name: "Mountain", Count: 4}, {Name: "Swamp", Count: 5}, {Name: "Plains", Count: 2},
	},
}

// TestMorophonLowers1 and TestMorophonLowers2 split the 200 seeds so each
// fits the 60 s test budget.
func TestMorophonLowers1(t *testing.T) {
	t.Parallel()
	lockstepSetRange(t, morophonPool, morophonDecks, 1, 100)
}

func TestMorophonLowers2(t *testing.T) {
	t.Parallel()
	lockstepSetRange(t, morophonPool, morophonDecks, 101, 200)
}

// TestMorophonDecksPlayable names a refused carrier here, not only by a
// stall far into the games.
func TestMorophonDecksPlayable(t *testing.T) {
	t.Parallel()
	_, set, gd := lockstepPool(t, morophonPool, morophonDecks)
	for s, d := range gd {
		if !SetDeckSupported(set, d) {
			t.Errorf("seat %d: deck not playable in the %s pool", s, morophonPool)
		}
	}
}

// TestMorophonMutation: with the colour reduction's shortfall spilling to
// generic again, some seed diverges, so the pool casts a spell of the chosen
// type under Morophon, Edgewalker or Ragemonger with a generic part the reduction must not touch.
// It is not parallel: it sets the kernel's DebugReduceNoSpillFlip.
func TestMorophonMutation(t *testing.T) {
	dirac.DebugReduceNoSpillFlip = true
	defer func() { dirac.DebugReduceNoSpillFlip = false }()
	if lockstepRun(t, morophonPool, morophonDecks, 200, false, nil) == 0 {
		t.Error("spill: no seed diverged: the pool never prices a spell under a colour-only reduction")
	}
}
