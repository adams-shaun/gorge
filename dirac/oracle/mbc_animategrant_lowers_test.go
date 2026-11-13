package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The MBC AnimateAll Abilities$ grant family in lockstep against gorge, one
// deck pair per card (pool dirac/cardset/pools/fixtures/mbcanimate1):
//
//   - Rain of Filth ("until end of turn, lands you control gain 'Sacrifice
//     this land: Add {B}'"): the grant costs no {T}, so a land the seat has
//     already tapped still funds a play, and the sacrifice is the prefix
//     search's step (CR 602.2, 605.1a; dirac mana_grantcost.go grantSacDo).
//   - Song of Freyalise's saga chapters ("until your next turn, creatures
//     you control gain '{T}: Add one mana of any color'"): the granted {T}
//     needs the creature untapped and not summoning-sick, and the record's
//     lifetime is the controller's next turn (ovRec's ovNT).
//
// The grantmana subtest first asserts each deck is playable naming the card,
// then plays the lockstep games and asserts the kernel's counter for the
// granted activations moved, so a pool that never reaches the mechanism
// cannot pass; with the lowering reverted the card is refused and both
// assertions fail.

const mbcAnimate1Pool = "mbcanimate1"

// mbcRainDecks: seat 0's lands are mostly Mountains, so its black spells
// need the sacrificed Swamps (tapped ones included); seat 1 plays Forests.
var mbcRainDecks = [2][]cardset.Entry{
	{
		{Name: "Rain of Filth", Count: 4}, {Name: "Walking Corpse", Count: 8},
		{Name: "Terror", Count: 4}, {Name: "Mountain", Count: 16}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Forest", Count: 26},
	},
}

// mbcSongDecks: seat 1's lands are mostly Plains, so its green creatures
// need the tapped creatures the saga's grant pays with; seat 0 plays
// Mountains.
var mbcSongDecks = [2][]cardset.Entry{
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Mountain", Count: 26},
	},
	{
		{Name: "Song of Freyalise", Count: 4}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Hill Giant", Count: 4}, {Name: "Plains", Count: 16}, {Name: "Forest", Count: 8},
	},
}

var mbcAnimate1Decks = []struct {
	card     string
	decks    [2][]cardset.Entry
	deltaOf  func() int64
	minDelta int64
	what     string
}{
	{"Rain of Filth", mbcRainDecks, func() int64 { return dirac.GrantManaStats.Sac.Load() }, 10,
		"granted sacrifice-only mana steps (Rain of Filth)"},
	{"Song of Freyalise", mbcSongDecks, func() int64 { return dirac.GrantManaStats.Taps.Load() }, 10,
		"granted {T} mana steps (Song of Freyalise)"},
}

func TestMBCAnimate1Lowers(t *testing.T) {
	t.Parallel()
	t.Run("grantmana", func(t *testing.T) {
		t.Parallel()
		for _, tc := range mbcAnimate1Decks {
			t.Run(tc.card, func(t *testing.T) {
				t.Parallel()
				before := tc.deltaOf()
				lockstepSet(t, mbcAnimate1Pool, tc.decks, 128, "KMBCANIMATE1_SEEDS")
				if n := tc.deltaOf() - before; n < tc.minDelta {
					t.Fatalf("%s: %d (< %d): the pool never reaches the mechanism", tc.what, n, tc.minDelta)
				} else {
					t.Logf("%s: %d", tc.what, n)
				}
			})
		}
	})
}

// TestMBCAnimate1DecksPlayable asserts each family card is playable in the
// pool's set on its own, so a refused card is named here and not only by a
// stall far into the games.
func TestMBCAnimate1DecksPlayable(t *testing.T) {
	t.Parallel()
	for _, tc := range mbcAnimate1Decks {
		_, set, gd := lockstepPool(t, mbcAnimate1Pool, tc.decks)
		for s, d := range gd {
			if !SetDeckSupported(set, d) {
				t.Errorf("seat %d: %s not playable in the %s pool", s, tc.card, mbcAnimate1Pool)
			}
		}
	}
}
