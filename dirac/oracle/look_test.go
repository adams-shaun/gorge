package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The private-look lockstep: gorge paces a mandatory private
// look with a one-option "Continue" ask to the looker (effects poseLookAck)
// whenever the looked-at hand or library top holds a card -- Gitaxian Probe,
// Peek, Telepathic Spies, Wu Scout and Glasses of Urza look at a hand,
// Mishra's Bauble, Orcish Spy, Wand of Denial at a library -- and Mind Rot
// empties hands so the empty-pool look poses nothing. Single-option
// decisions are not elided, so every ack is a decision both sides pose.
var klookDecks = [2][]cardset.Entry{
	{
		{Name: "Gitaxian Probe", Count: 4}, {Name: "Mishra's Bauble", Count: 4}, {Name: "Peek", Count: 3},
		{Name: "Telepathic Spies", Count: 3}, {Name: "Glasses of Urza", Count: 2}, {Name: "Wind Drake", Count: 4},
		{Name: "Shock", Count: 3}, {Name: "Island", Count: 11}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Mind Rot", Count: 4}, {Name: "Orcish Spy", Count: 4}, {Name: "Wand of Denial", Count: 2},
		{Name: "Wu Scout", Count: 3}, {Name: "Clairvoyance", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 6},
	},
}

func TestLookLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "look", klookDecks, 1000, "KLOOK_SEEDS")
}

// TestLookLockstepMutation proves the pool poses the new acks: without
// them, some seed diverges.
func TestLookLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoLookAck = true
	defer func() { dirac.DebugNoLookAck = false }()
	if !lockstepDiverges(t, "look", klookDecks, 40) {
		t.Fatal("no seed diverged without the look_ack: the pool never looks at a non-empty hand or library")
	}
}
