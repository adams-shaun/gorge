package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The MBC replacements-1 family in lockstep against gorge, one deck pair per
// spelling kgen now lowers (pool dirac/cardset/pools/fixtures/mbcrepl1):
//
//   - repl:DamageDone replacement shape: "if damage would be dealt to
//     CARDNAME, prevent that damage. Remove a +1/+1 counter from CARDNAME"
//     (Phantom Tiger, Phantom Centaur, Phantom Flock): CR 615, once per
//     damage event, ReplPreventDamageCounters with ReplPrevSelfRemoveOne.
//   - repl:DamageDone ActiveZones: a bodyless Prevent$ True on damage to
//     itself with no ActiveZones$ (Istvan, Butcher of Eln: from creatures on
//     the battlefield; Guardian Naga: during your turn),
//     ReplPreventDamageToSelf.
//   - other:replacement unread params [ActiveZones IsPresent]: Alirios,
//     Enraptured doesn't untap during your untap step while you control a
//     Reflection (CR 502.3 with an IsPresent$ gate), ReplDoesntUntap with a
//     Repl.Cond.
//
// Each subtest first asserts the deck is playable naming the card (the
// lowering reverted, the card is refused and this fails), then plays the
// lockstep games and asserts the kernel's counter for the mechanism moved,
// so a pool that never reaches the mechanism cannot pass.

const mbcRepl1Pool = "mbcrepl1"

var mbcPhantomDecks = [2][]cardset.Entry{
	{
		{Name: "Phantom Tiger", Count: 7}, {Name: "Phantom Centaur", Count: 7},
		{Name: "Forest", Count: 26},
	},
	{
		{Name: "Phantom Flock", Count: 4}, {Name: "Hill Giant", Count: 6}, {Name: "Shock", Count: 6},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 14},
	},
}

var mbcSelfPreventDecks = [2][]cardset.Entry{
	{
		{Name: "Istvan, Butcher of Eln", Count: 8}, {Name: "Guardian Naga", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Swamp", Count: 16}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 12},
	},
}

var mbcAliriosDecks = [2][]cardset.Entry{
	{
		{Name: "Alirios, Enraptured", Count: 10}, {Name: "Island", Count: 24}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 2},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Forest", Count: 26},
	},
}

// mbcRepl1Decks is the card each spelling must make playable, with the deck
// pair that plays it and the kernel counter for the mechanism.
var mbcRepl1Decks = []struct {
	cards    []string
	decks    [2][]cardset.Entry
	deltaOf  func() int64
	minDelta int64
	what     string
}{
	{[]string{"Phantom Tiger", "Phantom Centaur", "Phantom Flock"}, mbcPhantomDecks,
		func() int64 { return dirac.SelfDamageStats.RemoveOne.Load() }, 10,
		"prevent-and-remove-a-counter bodies run"},
	{[]string{"Istvan, Butcher of Eln", "Guardian Naga"}, mbcSelfPreventDecks,
		func() int64 { return dirac.PrevToSelfStats.Hits.Load() }, 10,
		"self preventions applied"},
	{[]string{"Alirios, Enraptured"}, mbcAliriosDecks,
		func() int64 { return dirac.UntapCondStats.Held.Load() }, 10,
		"conditional doesn't-untap gates that held"},
}

func TestMBCReplacements1Lowers(t *testing.T) {
	t.Parallel()
	for _, tc := range mbcRepl1Decks {
		t.Run(tc.cards[0], func(t *testing.T) {
			t.Parallel()
			_, set, gd := lockstepPool(t, mbcRepl1Pool, tc.decks)
			for s, d := range gd {
				if !SetDeckSupported(set, d) {
					t.Fatalf("seat %d: %v not playable in the %s pool", s, tc.cards, mbcRepl1Pool)
				}
			}
			before := tc.deltaOf()
			lockstepSet(t, mbcRepl1Pool, tc.decks, 128, "KMBCREPL1_SEEDS")
			if n := tc.deltaOf() - before; n < tc.minDelta {
				t.Fatalf("%s: %d (< %d): the pool never reaches the mechanism", tc.what, n, tc.minDelta)
			} else {
				t.Logf("%s: %d", tc.what, n)
			}
		})
	}
}
