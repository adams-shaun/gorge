package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestMBCKeywords4Lowers: the Mystery Booster Commander keywords-4 spelling
// family in lockstep, one subtest per lowered spelling over the
// mbckeywordsfour fixture pool.
//
//   - saddle: Jandor, Fortuned Traveler's static grants saddle 2 (CR 702.171a)
//     to the Beasts, Camels, Horses, Elephants and Wolves its controller
//     controls and makes them Mounts, so each gets its own activated saddle
//     ability; the probe counts the saddle activations the games reach, and
//     fails when none is reached.
func TestMBCKeywords4Lowers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		decks  [2][]cardset.Entry
		seeds  uint64
		saddle int // the least saddle activations the games must reach
	}{
		{"saddle", mbcSaddleDecks, 120, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			saddles := 0
			pol := func(seed uint64) Policy { return mbcSaddleProbe(castFirstPolicy(seed), &saddles) }
			if div := lockstepRunPolicy(t, "mbckeywordsfour", tc.decks, tc.seeds, true, nil, nil, pol); div != 0 {
				return // each divergence is already a test error
			}
			t.Logf("%s: %d saddle activations", tc.name, saddles)
			if saddles < tc.saddle {
				t.Fatalf("%s: %d saddle activations (want %d) over %d seeds: the pool does not exercise it",
					tc.name, saddles, tc.saddle, tc.seeds)
			}
		})
	}
}

// mbcSaddleProbe wraps base and counts, when gorge leads, the Saddle markers
// its log gains.
func mbcSaddleProbe(base Policy, saddles *int) Policy {
	seen := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		evs := c.G.E.L.Events
		for ; seen < len(evs); seen++ {
			if evs[seen].Kind == events.Saddle {
				*saddles++
			}
		}
		return a
	})
}

// mbcSaddleDecks: Jandor to grant, Mounts to saddle, bodies to tap for it.
var mbcSaddleDecks = [2][]cardset.Entry{
	{
		{Name: "Jandor, Fortuned Traveler", Count: 6}, {Name: "Armored Warhorse", Count: 6}, {Name: "Axebane Beast", Count: 6},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Jandor, Fortuned Traveler", Count: 4}, {Name: "Armored Warhorse", Count: 4}, {Name: "Axebane Beast", Count: 4},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
}
