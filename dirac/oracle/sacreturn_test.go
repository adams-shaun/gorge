package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Each player's sacrifice moves as that player finishes, but the batch ends with
// the last player's: a card a sacrificed Bat exiled stays in exile while the next
// player chooses.
func TestSacrificedHostReturnsAfterTheWholeBatch(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Deep-Cavern Bat", Count: 10}, {Name: "Walking Corpse", Count: 8},
		{Name: "Barter in Blood", Count: 6}, {Name: "Swamp", Count: 16},
	}
	lockstepRun(t, "sacreturn", [2][]cardset.Entry{deck, deck}, 100, true, nil)
}
