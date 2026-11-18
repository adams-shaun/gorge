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

// TestRavenloftLockstep: the kernel puts Ravenloft Adventurer's Attacks
// trigger on the stack once its controller has completed a dungeon
// (DungeonsCompleted$Amount, genState's coldDDone), as the card reads, and
// pays by the defender-owned exile count
// (TriggeredDefendingPlayer$ValidExile Card.YouOwn+counters_GE1_HIT). Gorge
// reads both heads (engine/effects count_body_player.go's DungeonsCompleted$
// arm, count_ref_property.go's TriggeredDefendingPlayer Valid-family widening),
// so the pool is strict lockstep like TestInitLockstep.
func TestRavenloftLockstep(t *testing.T) {
	t.Parallel()
	lockstepRun(t, "ravenloft", kravenDecks, 200, true, nil)
}
