package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Gaze of Pain and Delif's Cone lockstep (dirac/kgen/lower_zone.go emblem over
// an end-of-turn Mode$ AttackerUnblocked line; kernel trig_emblemunblocked.go
// fireEmblemsUnblocked): "whenever a creature you control attacks and isn't
// blocked, you may have it deal damage equal to its power to a target
// creature, then it assigns no combat damage" and the Cone's captured-attacker
// form with a life gain. The deck is vanilla 2/2 attackers beside the two cards,
// so the emblems are live when blockers are declared.
func effUnblockedDecks() [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: "Gaze of Pain", Count: 6}, {Name: "Delif's Cone", Count: 6}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Forest", Count: 9}, {Name: "Swamp", Count: 9},
	}
	return [2][]cardset.Entry{a, append([]cardset.Entry(nil), a...)}
}

func TestEffUnblockedLockstep(t *testing.T) {
	st := &dirac.EffEOTTrigStats{}
	dirac.DebugEffEOTTrigStats = st
	defer func() { dirac.DebugEffEOTTrigStats = nil }()
	lockstepRun(t, "effunblocked", effUnblockedDecks(), 300, true, nil)
	if st.UnblockedFires == 0 {
		t.Fatal("no unblocked emblem trigger was queued in the games; the lockstep proves nothing")
	}
	t.Logf("unblocked emblem fires %d", st.UnblockedFires)
}

// TestEffUnblockedLockstepMutation: with the unblocked emblem hook off, some
// seed diverges (gorge fires the line; the kernel would not).
func TestEffUnblockedLockstepMutation(t *testing.T) {
	dirac.DebugIgnoreUnblockedEmblem = true
	defer func() { dirac.DebugIgnoreUnblockedEmblem = false }()
	if lockstepRun(t, "effunblocked", effUnblockedDecks(), 300, false, nil) == 0 {
		t.Fatal("ignoring the unblocked emblem left the lockstep clean")
	}
}
