package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Play Amount$/Defined$ population lockstep
// (dirac/kgen/lower_playpopcount.go): Plargg and Nassari's upkeep trigger
// exiles from each library until a nonland card, the opponent chooses one,
// and its controller may cast up to two of the others free (Defined$
// Remembered.!ChosenCard, Amount$ 2: CastFree's UpTo bit over the remembered
// set with the chosen card filtered out), the casts walking either player's
// exiled cards.
// Harness the Storm's Play Defined$ Targeted joins the deck: two of its
// triggers share one graveyard card, and a paid cast the pool cannot cover is
// reversed with the card still a legal target for the second trigger.
// Run as its own shard (-run '^TestPlayPopCountLockstep$').
func playPopCountDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Plargg and Nassari", Count: 4},
		{Name: "Harness the Storm", Count: 4},
		{Name: "Shock", Count: 4},
		{Name: "Lightning Strike", Count: 4},
		{Name: "Volcanic Hammer", Count: 3},
		{Name: "Raging Goblin", Count: 4},
		{Name: "Goblin Piker", Count: 3},
		{Name: "Lava Axe", Count: 3},
		{Name: "Mountain", Count: 11},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

func TestPlayPopCountLockstep(t *testing.T) {
	t.Parallel()
	tally := new(psTally)
	lockstepRunPolicy(t, "playpopcount", playPopCountDecks(), 200, true, nil, nil, playShapesPolicy(tally))
	t.Logf("free plays %d (Plargg and Nassari)", tally.free)
	if tally.free < 20 {
		t.Errorf("free plays %d, want at least 20; the lockstep proves nothing", tally.free)
	}
}
