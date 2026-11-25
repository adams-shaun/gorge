package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The staticzones round's lowered MayPlay shapes that do not fit the main
// pool's decks (fixture staticzones, dirac/oracle/staticzones_test.go), each
// with exactly one alternative/free static family per controller: the
// kernel's canon cannot name a second alternative index beside one
// (StallZeroAltShared), so the priced alternatives (Nissa, Worldsoul
// Speaker's eight energy, Demon of Fate's Design's mana value in life) run
// in one mirror; the free family (Dracogenesis) rides the main pool's decks.
//
// kNissaDecks: Nissa's landfall feeds the eight energy; the alternative
// applies to any permanent spell its controller casts. kDemonDecks: the
// Demon pays an enchantment spell's mana value in life (its Condition$
// PlayerTurn and MayPlayLimit$ 1 ride the same static). Burn and cheap
// creatures pace the mirror so the priced alternatives' resources (energy,
// life) matter. The Demon runs one copy: a second on the battlefield is a
// second alternative static (StallZeroAltShared); Nissa is legendary, so one
// stays.
var kNissaDecks = [2][]cardset.Entry{
	{
		{Name: "Nissa, Worldsoul Speaker", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Slithering Shade", Count: 2}, {Name: "Spire Serpent", Count: 2},
		{Name: "Giant Growth", Count: 3},
		{Name: "Forest", Count: 25}, {Name: "Plains", Count: 3},
	},
	{
		{Name: "Demon of Fate's Design", Count: 1},
		{Name: "Ivory Mask", Count: 2}, {Name: "Solitary Confinement", Count: 2},
		{Name: "Power Artifact", Count: 2}, {Name: "Case of the Locked Hothouse", Count: 2},
		{Name: "Lightning Bolt", Count: 3},
		{Name: "Swamp", Count: 17}, {Name: "Mountain", Count: 11},
	},
}

// TestStaticZonesMayPlayAltLockstep plays the priced alternatives' mirror
// lockstep with gorge.
func TestStaticZonesMayPlayAltLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "staticzones", kNissaDecks, 200, "KSTATICZONESALT_SEEDS")
}
