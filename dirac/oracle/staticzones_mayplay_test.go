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
// Speaker's eight energy, Demon of Fate's Design's mana value in life,
// Conspiracy Unraveler's ten evidence) run in mirrors of their own; the free
// family (Dracogenesis, Squee, the Immortal's self grant) rides the main
// pool's decks, and a free plain cast beside a priced alternative would
// share the cast's mode (StallZeroAltShared), so the families do not share a
// deck either.
//
// kNissaDecks: Nissa's landfall feeds the eight energy; the alternative
// applies to any permanent spell its controller casts. kDemonDecks: the
// Demon pays an enchantment spell's mana value in life (its Condition$
// PlayerTurn and MayPlayLimit$ 1 ride the same static). Burn and cheap
// creatures pace the mirror so the priced alternatives' resources (energy,
// life) matter. The Demon runs one copy: a second on the battlefield is a
// second alternative static (StallZeroAltShared); Nissa is legendary, so one
// stays.
//
// kUnravelerDecks: Conspiracy Unraveler's alternative applies to any spell
// its controller casts, paid by exiling picked cards worth ten from the
// seat's own graveyard (CR 701.59a): the seat's burn kills its own small
// bodies into that graveyard and its spells join it, so the payment has
// candidates while the seat's Islands cast the Sphinx itself.
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

// kUnravelerDecks is the collect-evidence alternative's mirror: seat 0 holds
// Conspiracy Unraveler (one copy: a second on the battlefield is a second
// alternative static, StallZeroAltShared) and fills its own graveyard;
// seat 1's removal keeps the Sphinx honest.
var kUnravelerDecks = [2][]cardset.Entry{
	{
		{Name: "Conspiracy Unraveler", Count: 1},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Dragon's Rage Channeler", Count: 2}, {Name: "Spire Serpent", Count: 2},
		{Name: "Island", Count: 16}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Swords to Plowshares", Count: 3}, {Name: "Giant Growth", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "True Believer", Count: 2}, {Name: "Trusted Advisor", Count: 2},
		{Name: "Lightning Bolt", Count: 3},
		{Name: "Plains", Count: 14}, {Name: "Mountain", Count: 10},
	},
}

// TestStaticZonesMayPlayAltLockstep plays the priced alternatives' mirror
// lockstep with gorge.
func TestStaticZonesMayPlayAltLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "staticzones", kNissaDecks, 200, "KSTATICZONESALT_SEEDS")
}

// TestStaticZonesMayPlayEvidenceLockstep plays the collect-evidence
// alternative's mirror lockstep with gorge.
func TestStaticZonesMayPlayEvidenceLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "staticzones", kUnravelerDecks, 200, "KSTATICZONESEV_SEEDS")
}
