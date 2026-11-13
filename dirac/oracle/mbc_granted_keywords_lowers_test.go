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
//   - livingweapon: Ekthi, Contaminator Priest's static gives each Equipment
//     its controller has living weapon (CR 702.92a, 613.1f), so an Equipment
//     that enters under Ekthi makes a Germ and attaches to it; the probe
//     counts the Germs the games make, and fails when none is made.
func TestMBCKeywords4Lowers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		decks  [2][]cardset.Entry
		seeds  uint64
		saddle int // the least saddle activations the games must reach
		germs  int // the least granted-living-weapon Germs the games must make
	}{
		{"saddle", mbcSaddleDecks, 120, 20, 0},
		{"livingweapon", mbcLivingWeaponDecks, 120, 0, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			saddles, germs := 0, 0
			pol := func(seed uint64) Policy {
				return mbcEventProbe(castFirstPolicy(seed), &saddles, &germs)
			}
			if div := lockstepRunPolicy(t, "mbckeywordsfour", tc.decks, tc.seeds, true, nil, nil, pol); div != 0 {
				return // each divergence is already a test error
			}
			t.Logf("%s: %d saddle activations, %d Germs", tc.name, saddles, germs)
			if saddles < tc.saddle || germs < tc.germs {
				t.Fatalf("%s: %d saddle activations (want %d), %d Germs (want %d) over %d seeds: the pool does not exercise it",
					tc.name, saddles, tc.saddle, germs, tc.germs, tc.seeds)
			}
		})
	}
}

// mbcEventProbe wraps base and counts, when gorge leads, the Saddle markers
// its log gains and the Phyrexian Germ tokens it mints.
func mbcEventProbe(base Policy, saddles, germs *int) Policy {
	seen := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil {
			return a
		}
		evs := c.G.E.L.Events
		for ; seen < len(evs); seen++ {
			switch ev := evs[seen]; {
			case ev.Kind == events.Saddle:
				*saddles++
			case ev.Kind == events.TokenCreate && ev.Text == "b_0_0_phyrexian_germ":
				*germs++
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

// mbcLivingWeaponDecks: Ekthi to grant, Equipment to enter under it, bodies
// to cast the rest.
var mbcLivingWeaponDecks = [2][]cardset.Entry{
	{
		{Name: "Ekthi, Contaminator Priest", Count: 6}, {Name: "Short Sword", Count: 6}, {Name: "Bonesplitter", Count: 6},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Ekthi, Contaminator Priest", Count: 4}, {Name: "Short Sword", Count: 4}, {Name: "Bonesplitter", Count: 4},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
}
