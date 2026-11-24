package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The AtEOT$ Exile group lockstep: Feral Lightning's three hasty Elementals
// ("exile them at the beginning of the next end step") are one delayed
// triggered ability naming all three (gorge scheduleAtEOTGroup, CR 603.7a/b;
// the kernel's exileAtEndJoin followers), so the end step puts one trigger on
// the stack, not three, and no trigger-order ask is posed.
var kexileGroupDecks = [2][]cardset.Entry{
	{{Name: "Feral Lightning", Count: 12}, {Name: "Mountain", Count: 28}},
	{{Name: "Grizzly Bears", Count: 12}, {Name: "Forest", Count: 28}},
}

func TestAtEOTExileGroupLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "ateotexilegroup", kexileGroupDecks, 64, "KEXILEGROUP_SEEDS")
}

// TestAtEOTExileGroupMutation: every token registering its own exile trigger
// (the pre-group shape) must diverge from gorge's one trigger.
func TestAtEOTExileGroupMutation(t *testing.T) {
	dirac.DebugExileGroupOff = true
	defer func() { dirac.DebugExileGroupOff = false }()
	if !lockstepDiverges(t, "ateotexilegroup", kexileGroupDecks, 48) {
		t.Fatal("no seed diverged with the exile group off: the pool does not exercise Feral Lightning's grouped trigger")
	}
}
