package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// kravenDecks is kinitDecks with Ravenloft Adventurer in the first deck: its
// enters trigger takes the initiative (so the venture completes dungeons), its
// replacement exiles the opponent's dying creatures with a hit counter, and
// its Attacks trigger reads the completed-dungeon count.
var kravenDecks = func() [2][]cardset.Entry {
	d := [2][]cardset.Entry{append([]cardset.Entry(nil), kinitDecks[0]...), kinitDecks[1]}
	for i := range d[0] {
		if d[0][i].Name == "Hill Giant" {
			d[0][i] = cardset.Entry{Name: "Ravenloft Adventurer", Count: 3}
		}
	}
	return d
}()

// TestRavenloftAdventurerLockstepKnownGap: the kernel puts Ravenloft
// Adventurer's Attacks trigger on the stack once its controller has completed
// a dungeon (DungeonsCompleted$Amount, genState's coldDDone), as the card
// reads; gorge never does, because its count evaluator has no
// DungeonsCompleted$ head (it reads 0) and no TriggeredDefendingPlayer$
// ValidExile head. So the pool diverges on exactly that stack entry until
// gorge reads both heads; then this test fails and becomes a lockstepRun with
// report set, like TestInitLockstep.
func TestRavenloftAdventurerLockstepKnownGap(t *testing.T) {
	t.Parallel()
	if !lockstepDiverges(t, "ravenloft", kravenDecks, 120) {
		t.Fatal("no seed diverged: gorge now reads DungeonsCompleted$Amount; make this a strict lockstepRun")
	}
}
