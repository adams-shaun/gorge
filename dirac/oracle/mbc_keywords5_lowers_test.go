package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The MBC keywords-5 family in lockstep against gorge (pool
// dirac/cardset/pools/fixtures/mbckeywords5): an Effect's Mode$ Continuous
// static granting "Prevent all combat damage that would be dealt by
// CARDNAME." to every nongreen creature until end of turn (Hunter's Ambush;
// gorge rules/replacement_prevent_kw.go's by-combat head, the kernel's
// KwPreventCombatDamageFrom read by preventFromGen).
//
// Renown, Mutate and Exploit (Consul's Lieutenant, Dreamtail Heron, Fell
// Stinger) stay refused: each card's other blockers are its own families.
//
// The subtest asserts the deck is playable naming the card, then plays the
// lockstep games and asserts the kernel prevented combat damage through the
// granted bit, so a pool that never reaches the mechanism cannot pass; with
// the lowering reverted the card is refused and the assertion fails.

const mbcKeywords5Pool = "mbckeywords5"

var mbcHuntersAmbushDecks = [2][]cardset.Entry{
	{
		{Name: "Hunter's Ambush", Count: 8}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Forest", Count: 24},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Savannah Lions", Count: 8},
		{Name: "Mountain", Count: 12}, {Name: "Plains", Count: 12},
	},
}

var mbcKeywords5Decks = []struct {
	card     string
	decks    [2][]cardset.Entry
	deltaOf  func() int64
	minDelta int64
	what     string
}{
	{"Hunter's Ambush", mbcHuntersAmbushDecks, func() int64 { return dirac.PrevFromStats.Events.Load() }, 10,
		"combat damage events prevented from a granted source"},
}

func TestMBCKeywords5Lowers(t *testing.T) {
	t.Parallel()
	for _, tc := range mbcKeywords5Decks {
		t.Run(tc.card, func(t *testing.T) {
			_, set, gd := lockstepPool(t, mbcKeywords5Pool, tc.decks)
			for s, d := range gd {
				if !SetDeckSupported(set, d) {
					t.Fatalf("seat %d: %s not playable in the %s pool", s, tc.card, mbcKeywords5Pool)
				}
			}
			before := tc.deltaOf()
			lockstepSet(t, mbcKeywords5Pool, tc.decks, 128, "KMBCKEYWORDS5_SEEDS")
			if n := tc.deltaOf() - before; n < tc.minDelta {
				t.Fatalf("%s: %d (< %d): the pool never reaches the mechanism", tc.what, n, tc.minDelta)
			} else {
				t.Logf("%s: %d", tc.what, n)
			}
		})
	}
}
