package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The MBC tokens-1 family in lockstep against gorge (pool
// dirac/cardset/pools/fixtures/mbctokens1):
//
//   - a comma-listed TokenScript$ beside TokenOwner$ You, the controller every
//     list mints for already (Bestial Menace: a Snake, a Wolf and an Elephant).
//   - a token gated by a second presence group, ConditionPresent$ beside
//     ConditionPresent2$ (Banishing Slash: "if you control an artifact and an
//     enchantment"); gorge's conditionMet requires both legs, the deck holds
//     artifacts and Auras so the gate is sometimes met and sometimes not.
//
// Each card is first asserted playable in the pool's set (a refused card fails
// there, naming it), then the lockstep games run clean.

const mbcTokens1Pool = "mbctokens1"

var mbcMenaceDecks = [2][]cardset.Entry{
	{
		{Name: "Bestial Menace", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Forest", Count: 26},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Savannah Lions", Count: 6},
		{Name: "Mountain", Count: 26},
	},
}

var mbcSlashDecks = [2][]cardset.Entry{
	{
		{Name: "Banishing Slash", Count: 8}, {Name: "Pacifism", Count: 6},
		{Name: "Sol Ring", Count: 3}, {Name: "Wayfarer's Bauble", Count: 4},
		{Name: "Savannah Lions", Count: 3}, {Name: "Plains", Count: 16},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Sol Ring", Count: 4}, {Name: "Pacifism", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

var mbcTokens1Decks = []struct {
	card  string
	decks [2][]cardset.Entry
}{
	{"Bestial Menace", mbcMenaceDecks},
	{"Banishing Slash", mbcSlashDecks},
}

func TestMBCTokens1Lowers(t *testing.T) {
	t.Parallel()
	for _, tc := range mbcTokens1Decks {
		t.Run(tc.card, func(t *testing.T) {
			t.Parallel()
			_, set, gd := lockstepPool(t, mbcTokens1Pool, tc.decks)
			for s, d := range gd {
				if !SetDeckSupported(set, d) {
					t.Fatalf("seat %d: %s not playable in the %s pool", s, tc.card, mbcTokens1Pool)
				}
			}
			lockstepSet(t, mbcTokens1Pool, tc.decks, 128, "KMBCTOKENS1_SEEDS")
		})
	}
}
