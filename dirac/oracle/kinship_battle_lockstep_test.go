package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The kinship / player-or-battle lockstep: Kinship's bare
// sharesCreatureTypeWith (the revealed top card shares a creature type with
// the source, gorge wordSharesCreatureType over Self) beside DamageDone
// triggers whose ValidTarget$ lists Player with one permanent type
// (Player,Battle: TfToPlayer beside a Filter2).
var kkinshipBattleDecks = [2][]cardset.Entry{
	{{Name: "Wolf-Skull Shaman", Count: 4}, {Name: "Winnower Patrol", Count: 4}, {Name: "Plane-Merge Elf", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Forest", Count: 24}},
	{{Name: "Archpriest of Shadows", Count: 4}, {Name: "Doomskar Warrior", Count: 4}, {Name: "Deeproot Wayfinder", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 8}},
}

func TestKinshipBattleLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "kinshipbattle", kkinshipBattleDecks, 200, "KKINSHIPBATTLE_SEEDS")
}
