package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The MBC statics-3 family in lockstep against gorge (pool
// dirac/cardset/pools/fixtures/mbcstatics3), one deck pair per spelling kgen
// now lowers:
//
//   - a CantGainLife static whose ValidPlayer$ is Opponent (Tibalt, Rakish
//     Instigator): gorge lifeGainForbidden matches the spec from the static's
//     controller, so the opponents' gains are swallowed and the controller's
//     are not (kop.StBanOpp). The decks give both seats lifegain.
//   - an Aura whose Continuous static carries AddSVar$ beside RemoveAllAbilities
//     and AddAbility (Heliod's Punishment): gorge grants no variable for a body
//     that is not in the "SVar:<Name>:<Value>" shape, so the parameter reads
//     nothing; the enchanted creature taps to remove a TASK counter.
//
// Each subtest first asserts the deck is playable naming the card; with the
// lowering reverted the card is refused and the assertion fails.

const mbcStatics3Pool = "mbcstatics3"

var mbcTibaltDecks = [2][]cardset.Entry{
	{
		{Name: "Tibalt, Rakish Instigator", Count: 6}, {Name: "Lone Missionary", Count: 6},
		{Name: "Healer's Hawk", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Lone Missionary", Count: 8}, {Name: "Healer's Hawk", Count: 6},
		{Name: "Soulmender", Count: 6}, {Name: "Savannah Lions", Count: 4},
		{Name: "Plains", Count: 16},
	},
}

var mbcHeliodDecks = [2][]cardset.Entry{
	{
		{Name: "Heliod's Punishment", Count: 8}, {Name: "Savannah Lions", Count: 6},
		{Name: "Plains", Count: 26},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Forest", Count: 14}, {Name: "Mountain", Count: 12},
	},
}

var mbcStatics3Cases = []struct {
	card  string
	decks [2][]cardset.Entry
}{
	{"Tibalt, Rakish Instigator", mbcTibaltDecks},
	{"Heliod's Punishment", mbcHeliodDecks},
}

func TestMBCStatics3Lowers(t *testing.T) {
	t.Parallel()
	for _, tc := range mbcStatics3Cases {
		t.Run(tc.card, func(t *testing.T) {
			t.Parallel()
			_, set, gd := lockstepPool(t, mbcStatics3Pool, tc.decks)
			for s, d := range gd {
				if !SetDeckSupported(set, d) {
					t.Fatalf("seat %d: %s not playable in the %s pool", s, tc.card, mbcStatics3Pool)
				}
			}
			lockstepSet(t, mbcStatics3Pool, tc.decks, 128, "KMBCSTATICS3_SEEDS")
		})
	}
}

// TestMBCStatics3BanMutation: with the player ban switched off the kernel
// gains life gorge swallows, so the Tibalt pair diverges; the lockstep above
// therefore reaches the opponent-scoped ban.
func TestMBCStatics3BanMutation(t *testing.T) {
	old := dirac.DebugPlayerBanOff
	dirac.DebugPlayerBanOff = true
	defer func() { dirac.DebugPlayerBanOff = old }()
	if !lockstepDiverges(t, mbcStatics3Pool, mbcTibaltDecks, 64) {
		t.Fatal("the Tibalt pair never reaches the opponent-scoped life gain ban")
	}
}
