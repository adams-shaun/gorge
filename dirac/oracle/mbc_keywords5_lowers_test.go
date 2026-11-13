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
// Renown 1 (Consul's Lieutenant; gorge cards/kw_renown.go's combat-damage
// trigger under Card.Self+!IsRenowned, its PutCounter Renown$ body, and the
// card's "if it's renowned" attack trigger): the kernel's OpExt ExtRenown and
// PredRenowned (dirac/act_renown.go).
//
// Mutate and Exploit (Dreamtail Heron, Fell Stinger) stay refused: each
// card's other blockers are its own families.
//
// Each subtest asserts the deck is playable naming the card, then plays the
// lockstep games and asserts the kernel reached the mechanism (prevented
// combat damage through the granted bit; made a permanent renowned), so a
// pool that never reaches it cannot pass; with the lowering reverted the card
// is refused and the assertion fails.

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

var mbcConsulsLieutenantDecks = [2][]cardset.Entry{
	{
		{Name: "Consul's Lieutenant", Count: 10}, {Name: "Savannah Lions", Count: 6},
		{Name: "Plains", Count: 24},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 14},
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
	{"Consul's Lieutenant", mbcConsulsLieutenantDecks, func() int64 { return dirac.RenownStats.Marked.Load() }, 10,
		"permanents made renowned"},
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
