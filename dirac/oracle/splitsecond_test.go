package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Samut, Tyrant of Naktamun gives its controller's instant and sorcery spells
// split second: while one is on the stack no spell is cast and no non-mana
// ability is activated (CR 702.61a). The opponent's responses and the Prodigal
// Sorcerers' pings are withheld from the offer, as gorge withholds them.
var splitsecondDecks = [2][]cardset.Entry{
	{
		{Name: "Samut, Tyrant of Naktamun", Count: 5}, {Name: "Lightning Bolt", Count: 6},
		{Name: "Shock", Count: 4}, {Name: "Prodigal Sorcerer", Count: 3},
		{Name: "Island", Count: 11}, {Name: "Mountain", Count: 11},
	},
	{
		{Name: "Unsummon", Count: 4}, {Name: "Essence Scatter", Count: 4}, {Name: "Prodigal Sorcerer", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

func TestSplitSecondGrantPlaysInLockstep(t *testing.T) {
	if n := lockstepRun(t, "splitsecond", splitsecondDecks, 200, true, nil); n != 0 {
		t.Fatalf("%d divergent games", n)
	}
}

// TestSplitSecondGrantMutation leaves the priority offer whole: some game
// diverges, so the pool reaches a response to a split second spell.
func TestSplitSecondGrantMutation(t *testing.T) {
	dirac.DebugNoSplitSecond = true
	defer func() { dirac.DebugNoSplitSecond = false }()
	if n := lockstepRun(t, "splitsecond", splitsecondDecks, 200, false, nil); n == 0 {
		t.Fatal("no game diverged with split second ignored: the pool does not reach a response to a granted split second spell")
	}
}
