package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The own-creature Taps watchers (Mode$ Taps over a non-Self ValidCard$ with
// no ValidPlayer$, any tapper): Hawkeye's Bow's "whenever equipped creature
// becomes tapped, it deals 1 damage to each opponent" -- the equipped
// creature's attack tap and its mana tap both fire (gorge tapsMatches fires
// on every real Tap event of the permanent, mana taps included; the kernel
// fires TrigTaps from its attack taps and from the granted mana ability's
// {T}, gen manaTaps). Captain America, Living Legend's FirstTime$ True line
// (the arena's per-slot tapped-this-turn mark) has its own repro and is not
// in this pool. The expected answer at every decision is gorge's.
var ktapsOwnDecks = [2][]cardset.Entry{
	{
		{Name: "Hawkeye's Bow", Count: 6}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Llanowar Elves", Count: 6}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 5},
		{Name: "Plains", Count: 15}, {Name: "Mountain", Count: 14},
	},
}

func TestTapsOwnCreatureLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "tapsown", ktapsOwnDecks, 200, "KTAPSOWN_SEEDS")
}
