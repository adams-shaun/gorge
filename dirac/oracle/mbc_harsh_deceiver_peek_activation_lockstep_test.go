package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Harsh Deceiver's {1} ability is AB$ PeekAndReveal | Cost$ 1 | NoReveal$ True
// — look at the top card of your library, no reveal ask. The offer gate must
// price the {1} activation the same way the kernel does on every priority
// (CR 701.20e: the look needs a card; with the library empty the activation
// stays legal and resolves into nothing — no look_ack ask, no note, on either
// engine). Codsworth, Handy Helper's {T}: Add {W}{W} restricted to Aura and
// Equipment spells sits beside it, so a priority's menu carries both a
// restricted-mana producer and the peek.

// khpeekDecks: a forty-card white deck — Harsh Deceiver is cast on turn four
// and activated every turn after, and the library runs dry a few turns later,
// so the peek's empty-window activation is reached with mana spare. The green
// side attacks into it harmlessly (a 1/4 blocks a 2/2).
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
