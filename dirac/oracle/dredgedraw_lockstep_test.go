package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Mental Note mills two and then draws; a Golgari Thug (Dredge 4) milled by
// it can replace that draw (CR 702.52a-b).
const dredgeDrawPool = "dredgedraw"

var dredgeDrawDecks = [2][]cardset.Entry{
	{
		{Name: "Mental Note", Count: 6}, {Name: "Golgari Thug", Count: 6},
		{Name: "Island", Count: 14}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Mental Note", Count: 6}, {Name: "Golgari Thug", Count: 6},
		{Name: "Island", Count: 14}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 8},
	},
}

func TestDredgeDrawLockstep(t *testing.T) {
	lockstepRunPolicy(t, dredgeDrawPool, dredgeDrawDecks, 40, true, nil, nil, castFirstPolicy)
}

// TestDredgeDrawLockstep4 is the same pool in a four-seat pod, random policy.
func TestDredgeDrawLockstep4(t *testing.T) {
	d := pod4Deck(repeatName("Mental Note", 6), repeatName("Golgari Thug", 6),
		repeatName("Island", 14), repeatName("Swamp", 6), repeatName("Mountain", 8))
	if n := pod4FixtureLockstep(t, dredgeDrawPool, [4][]string{d, d, d, d}, 60, true, nil); n != 0 {
		t.Fatalf("%d seeds diverged", n)
	}
}
