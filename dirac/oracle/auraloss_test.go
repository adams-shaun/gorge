package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// An Aura enters attached, so one that removes abilities silences its host's
// "whenever an enchantment you control enters" (CR 603.6a).
func TestAuraLossSilencesHostEnterTrigger(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := []cardset.Entry{
		{Name: "Unable to Scream", Count: 12}, {Name: "Entity Tracker", Count: 6}, {Name: "Scrabbling Skullcrab", Count: 6},
		{Name: "Island", Count: 12}, {Name: "Plains", Count: 4},
	}
	lockstepRun(t, "auraloss", [2][]cardset.Entry{deck, deck}, 12, true, nil)
}
