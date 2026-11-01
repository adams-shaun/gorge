package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// "Any number of target players each discard a card" asks the first player, then
// the next with those picks in the graveyard; the program waits on each answer.
func TestDiscardOverSeveralTargetPlayers(t *testing.T) {
	deck := []cardset.Entry{{Name: "Tinybones Joins Up", Count: 20}, {Name: "Swamp", Count: 20}}
	lockstepRun(t, "tgtdiscard", [2][]cardset.Entry{deck, deck}, 16, true, nil)
}
