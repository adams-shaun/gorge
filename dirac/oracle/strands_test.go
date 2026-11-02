package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Prismatic Strands lockstep: "prevent all damage that
// sources of the colour of your choice would deal this turn" (gorge
// effChooseColor, then effEffect's bodyless DamageDone Prevent$ replacement
// over ValidSource$ Card.ChosenColor) and its flashback by tapping an
// untapped white creature, against red burn and red and green creatures in
// combat. The expected answer at every decision is gorge's. Each effect
// keeps the colour chosen as it was created, so a flashback in the same
// turn adds a second colour beside the first
// (cr-effect-chosencolor-snapshot): the tests skip on an older gorge pin.
var kstrandsDecks = [2][]cardset.Entry{
	{
		{Name: "Prismatic Strands", Count: 8}, {Name: "Savannah Lions", Count: 10}, {Name: "Serra Angel", Count: 4},
		{Name: "Plains", Count: 18},
	},
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}

// gorgeChosenColorSnapshotTime is gorge's UTC commit time
// (cr-effect-chosencolor-snapshot; no exported probe surface).
const gorgeChosenColorSnapshotTime = "20261026111538"

func strandsPinned(t *testing.T) bool {
	t.Helper()
	if !gorgePinAtLeast(t, gorgeChosenColorSnapshotTime) {
		t.Skip("gorge pin predates 690ed17f0 (each Effect's own ChosenColor)")
		return false
	}
	return true
}

func TestStrandsLockstep(t *testing.T) {
	strandsPinned(t)
	t.Parallel()
	lockstepSet(t, "strands", kstrandsDecks, 1000, "KSTRANDS_SEEDS")
}

// TestStrandsLockstepMutation proves the pool prevents damage: with the
// prevention dropped (the colour still asked), some seed diverges.
func TestStrandsLockstepMutation(t *testing.T) {
	strandsPinned(t)
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevCol = true
	defer func() { dirac.DebugNoPrevCol = false }()
	if !lockstepDiverges(t, "strands", kstrandsDecks, 200) {
		t.Fatal("no seed diverged with the colour prevention dropped: the pool never prevents a chosen colour's damage")
	}
}

// TestStrandsLockstepRecolorMutation is the flashback-twice-in-different-
// colours case: with a second resolution moving the first effect to the new
// colour (gorge's reading before 690ed17f0), some seed diverges -- the pool
// casts Strands, then flashes it back in the same turn naming another colour.
func TestStrandsLockstepRecolorMutation(t *testing.T) {
	strandsPinned(t)
	if isolate(t) {
		return
	}
	dirac.DebugPrevColRecolor = true
	defer func() { dirac.DebugPrevColRecolor = false }()
	if !lockstepDiverges(t, "strands", kstrandsDecks, 1000) {
		t.Fatal("no seed diverged with earlier effects recoloured: the pool never resolves Strands twice a turn in two colours")
	}
}
