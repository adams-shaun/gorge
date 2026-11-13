package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The MBC animate-2 family in lockstep against gorge, one deck pair per
// spelling kgen now lowers (pool dirac/cardset/pools/fixtures/mbcanimate2):
//
//   - an Animate staticAbilities$ grant whose single body is one Mode$
//     Continuous self P/T set (Myth Realized, and Chimeric Mass / Svogthos,
//     the Racecourse Rampage of the same class): CR 613.7b on the registered
//     static, livePT.
//   - the same grant whose body is Mode$ CombatDamageToughness (Streetwise
//     Negotiator's Backup rider): CR 702.165, 510.1, grantedStaticFlag.
//
// Each subtest plays the lockstep games and asserts the kernel's counter for
// the mechanism moved, so a pool that never reaches the mechanism cannot pass;
// with the lowering reverted the card is refused and the games fail.

const mbcAnimate2Pool = "mbcanimate2"

var mbcLivePTDecks = [2][]cardset.Entry{
	{
		{Name: "Myth Realized", Count: 6}, {Name: "Pacifism", Count: 8},
		{Name: "Plains", Count: 26},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Forest", Count: 26},
	},
}

var mbcNegotiatorDecks = [2][]cardset.Entry{
	{
		{Name: "Streetwise Negotiator", Count: 6}, {Name: "Savannah Lions", Count: 6},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Forest", Count: 22},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Mountain", Count: 26},
	},
}

// mbcAnimate2Decks is the card each spelling must make playable, with the
// deck pair that plays it and the kernel counter for the mechanism.
var mbcAnimate2Decks = []struct {
	card     string
	decks    [2][]cardset.Entry
	deltaOf  func() int64
	minDelta int64
	what     string
}{
	{"Myth Realized", mbcLivePTDecks, func() int64 { return dirac.LivePTStats.Reads.Load() }, 10,
		"live P/T reads of Animate staticAbilities$ bodies"},
	{"Streetwise Negotiator", mbcNegotiatorDecks, func() int64 { return dirac.GrantedStaticStats.Hits.Load() }, 10,
		"override-granted static reads (CombatDamageToughness)"},
}

func TestMBCAnimate2Lowers(t *testing.T) {
	t.Parallel()
	for _, tc := range mbcAnimate2Decks {
		t.Run(tc.card, func(t *testing.T) {
			t.Parallel()
			before := tc.deltaOf()
			lockstepSet(t, mbcAnimate2Pool, tc.decks, 128, "KMBCANIMATE2_SEEDS")
			if n := tc.deltaOf() - before; n < tc.minDelta {
				t.Fatalf("%s: %d (< %d): the pool never reaches the mechanism", tc.what, n, tc.minDelta)
			} else {
				t.Logf("%s: %d", tc.what, n)
			}
		})
	}
}
