package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Harsh Deceiver's {1} ability is AB$ PeekAndReveal | Cost$ 1 | NoReveal$ True
// — look at the top card of your library, no reveal ask. When its controller's
// library is empty the activation resolves into nothing (gorge effReveal's
// empty peek window: no look_ack ask, no note), so the activation offers with
// no observable resolution, and the payment planner's offer gate must agree
// with the kernel's on every priority (CR 701.20e: the look needs a card; the
// activation itself stays legal and resolves into nothing).

// khpeekDecks: a sixteen-card white deck — Harsh Deceiver is cast on turn
// four and activated every turn after, and the library runs dry a few turns
// later, so the peek's empty-window activation is reached with mana spare.
// The green side attacks into it harmlessly (a 1/4 blocks a 2/2).
var khpeekDecks = [2][]cardset.Entry{
	{
		{Name: "Harsh Deceiver", Count: 4}, {Name: "Codsworth, Handy Helper", Count: 6}, {Name: "Plains", Count: 30},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 36},
	},
}

// TestMBCHarshDeceiverPeekLockstep replays the empty-library peek
// activation lockstep (CR 701.20e: the look needs a card; the activation
// itself stays legal and resolves into nothing).
func TestMBCHarshDeceiverPeekLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "hpeek", khpeekDecks, 40, "HPEEK_SEEDS")
}
