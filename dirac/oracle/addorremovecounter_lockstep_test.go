package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The AddOrRemoveCounter lockstep (kgen lower_addorremovecounter.go, kernel
// ctr_addorremove.go): Plague Boiler's activated ability is the named-kind
// shape (one remove-or-put election for its plague counter), Quarry Hauler's
// enters trigger the each-existing shape over a target permanent. The two
// carriers share a deck so the Boiler's upkeep puts plague counters on a
// permanent Quarry Hauler can target, exercising both the add and remove arms
// and, once the board grows, a multi-kind ask (a Boiler with plague counters
// is one kind; a Quarry Hauler that added another is two).
func kaddorDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Plague Boiler", Count: 4},
		{Name: "Quarry Hauler", Count: 4},
		{Name: "Dramatist's Puppet", Count: 4},
		{Name: "Forest", Count: 10},
		{Name: "Swamp", Count: 9},
		{Name: "Mountain", Count: 9},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

func TestAddOrRemoveCounterLockstep(t *testing.T) {
	t.Parallel()
	asked := dirac.AddOrRemoveCounterStats.Asked.Load()
	each := dirac.AddOrRemoveCounterStats.EachExisting.Load()
	removed := dirac.AddOrRemoveCounterStats.Removed.Load()
	put := dirac.AddOrRemoveCounterStats.Put.Load()
	lockstepSet(t, "addorremovecounter", kaddorDecks(), 200, "KADDR_SEEDS")
	if dirac.AddOrRemoveCounterStats.Asked.Load() == asked {
		t.Fatal("no AddOrRemoveCounter election was posed: the pool does not exercise the op")
	}
	if dirac.AddOrRemoveCounterStats.EachExisting.Load() == each {
		t.Fatal("no each-existing election was posed: the pool does not exercise Quarry Hauler")
	}
	if dirac.AddOrRemoveCounterStats.Removed.Load() == removed || dirac.AddOrRemoveCounterStats.Put.Load() == put {
		t.Fatal("an arm was never answered: the pool does not exercise both remove and put")
	}
	t.Logf("AddOrRemoveCounter elections: asked %d, each-existing %d, remove %d, put %d",
		dirac.AddOrRemoveCounterStats.Asked.Load(),
		dirac.AddOrRemoveCounterStats.EachExisting.Load(),
		dirac.AddOrRemoveCounterStats.Removed.Load(),
		dirac.AddOrRemoveCounterStats.Put.Load())
}
