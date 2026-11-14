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
//   - escalate: Collective Resistance's modes (CR 702.120a) cost its escalate
//     {G} once for each mode chosen beyond the first, and gorge offers an
//     extra mode only while the floating pool pays for it (castable reads the
//     pool alone at the mode ask), so the cast the bot floats the printed cost
//     for is offered one mode; the probe counts the mode choices the games
//     reach, and fails when none is made.
func TestMBCKeywords4Lowers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		decks  [2][]cardset.Entry
		seeds  uint64
		saddle int // the least saddle activations the games must reach
		germs  int // the least granted-living-weapon Germs the games must make
		modes  int // the least Collective Resistance mode choices the games must make
	}{
		{"saddle", mbcSaddleDecks, 120, 20, 0, 0},
		{"livingweapon", mbcLivingWeaponDecks, 120, 0, 10, 0},
		{"escalate", mbcEscalateDecks, 120, 0, 0, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			saddles, germs, modes := 0, 0, 0
			pol := func(seed uint64) Policy {
				return mbcEventProbe(castFirstPolicy(seed), &saddles, &germs, &modes)
			}
			if div := lockstepRunPolicy(t, "mbckeywordsfour", tc.decks, tc.seeds, true, nil, nil, pol); div != 0 {
				return // each divergence is already a test error
			}
			t.Logf("%s: %d saddle activations, %d Germs, %d mode choices", tc.name, saddles, germs, modes)
			if saddles < tc.saddle || germs < tc.germs || modes < tc.modes {
				t.Fatalf("%s: %d saddle activations (want %d), %d Germs (want %d), %d mode choices (want %d) over %d seeds: the pool does not exercise it",
					tc.name, saddles, tc.saddle, germs, tc.germs, modes, tc.modes, tc.seeds)
			}
		})
	}
}

// mbcEventProbe wraps base and counts, when gorge leads, the Saddle markers
// its log gains, the Phyrexian Germ tokens it mints and the modes cast spells
// choose.
func mbcEventProbe(base Policy, saddles, germs, modes *int) Policy {
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
			case ev.Kind == events.ModeChosen && ev.Text != "":
				*modes++
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

// mbcEscalateDecks: Collective Resistance, an artifact, an enchantment and
// creatures to mode at, green mana (and the elves and petals that float it).
var mbcEscalateDecks = [2][]cardset.Entry{
	{
		{Name: "Collective Resistance", Count: 6}, {Name: "Short Sword", Count: 3}, {Name: "Bonesplitter", Count: 2},
		{Name: "Pacifism", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Llanowar Elves", Count: 3}, {Name: "Lotus Petal", Count: 5},
		{Name: "Forest", Count: 15},
	},
	{
		{Name: "Collective Resistance", Count: 6}, {Name: "Short Sword", Count: 3}, {Name: "Bonesplitter", Count: 2},
		{Name: "Pacifism", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3}, {Name: "Lotus Petal", Count: 5},
		{Name: "Forest", Count: 14},
	},
}
