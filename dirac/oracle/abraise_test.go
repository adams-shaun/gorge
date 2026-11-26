package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The activated-ability cost-raise lockstep, two pairs over one fixture pool
// (CR 601.2f, 602.2b; the expected answer at every decision, the offer, the
// autopay plan, the charge and the sacrifice ask, is gorge's):
//   - kabraiseBrutalDecks: Brutal Suppression's "Sacrifice a land" (one copy:
//     cardset.AbRaiseRefusal refuses a second) beside Rebels with plain
//     activated abilities, mana-costed (Rebel Informer's {3}, Mercenary
//     Informer's {2}{W}) and {T} (Errant Doomsayers, Blightspeaker);
//   - kabraiseRaysDecks: Oppressive Rays' "{3} more" on a creature with a
//     mana-costed activated ability (a {T} one is refused: abraiseTapDecks).
var kabraiseBrutalDecks = [2][]cardset.Entry{
	{
		{Name: "Brutal Suppression", Count: 1},
		{Name: "Rebel Informer", Count: 4}, {Name: "Errant Doomsayers", Count: 4},
		{Name: "Mercenary Informer", Count: 4}, {Name: "Blightspeaker", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Rebel Informer", Count: 4}, {Name: "Errant Doomsayers", Count: 4},
		{Name: "Mercenary Informer", Count: 4}, {Name: "Blightspeaker", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	},
}

var kabraiseRaysDecks = [2][]cardset.Entry{
	{
		{Name: "Oppressive Rays", Count: 4}, {Name: "Rebel Informer", Count: 6},
		{Name: "Mercenary Informer", Count: 6}, {Name: "Plains", Count: 12}, {Name: "Swamp", Count: 12},
	},
	{
		{Name: "Oppressive Rays", Count: 4}, {Name: "Rebel Informer", Count: 6},
		{Name: "Mercenary Informer", Count: 6}, {Name: "Plains", Count: 12}, {Name: "Swamp", Count: 12},
	},
}

// abraiseTapDecks pair Oppressive Rays with creatures whose ability taps.
var abraiseTapDecks = [2][]cardset.Entry{
	{{Name: "Oppressive Rays", Count: 4}, {Name: "Errant Doomsayers", Count: 8}, {Name: "Plains", Count: 28}},
	{{Name: "Blightspeaker", Count: 8}, {Name: "Swamp", Count: 32}},
}

// 60 seeds fit the 2-minute test cap; KABRAISE_SEEDS=1000 is the full sweep.
func TestAbraiseLockstep(t *testing.T) {
	t.Parallel()
	before := dirac.AbRaisedPriced.Load()
	lockstepSet(t, "abraise", kabraiseBrutalDecks, 60, "KABRAISE_SEEDS")
	lockstepSet(t, "abraise", kabraiseRaysDecks, 60, "KABRAISE_SEEDS")
	n := dirac.AbRaisedPriced.Load() - before
	t.Logf("%d ability pricings raised by a StatRaiseAbility static", n)
	if n == 0 {
		t.Fatal("no ability pricing was raised: the pool does not exercise the static")
	}
}

// TestAbraiseMutation proves the pool activates a raised ability: with the
// raise dropped (the generic and the Sacrifice a land), some seed diverges.
func TestAbraiseMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoAbRaise = true
	defer func() { dirac.DebugNoAbRaise = false }()
	if !lockstepDiverges(t, "abraise", kabraiseRaysDecks, 200) {
		t.Fatal("no seed diverged with the generic raise dropped: the pool never activates a raised ability")
	}
	if !lockstepDiverges(t, "abraise", kabraiseBrutalDecks, 400) {
		t.Fatal("no seed diverged with the sacrifice raise dropped: the pool never activates a Rebel under Brutal Suppression")
	}
}

// TestAbraiseTapRefused: a generic raise beside a creature whose ability taps
// is refused at the matchup (gorge withholds it under the raise).
func TestAbraiseTapRefused(t *testing.T) {
	testutil.CorpusRegistry(t) // first: without .cards the test skips before the corpus-built set loads
	set, err := cardset.RawByName("abraise")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := set.Matchup(abraiseTapDecks[0], abraiseTapDecks[1]); err == nil || !strings.Contains(err.Error(), "{T} ability") {
		t.Fatalf("Oppressive Rays beside a {T} creature: want the tapped-ability refusal, got %v", err)
	}
}
