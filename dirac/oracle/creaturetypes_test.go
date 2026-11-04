package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// An all-creature-types permanent lacks a subtype outside CR 205.3m: Undercover Skrull
// is no Plan, a type Doctor Doom reads in the same matchup.
func TestAllCreatureTypesLeaveOtherSubtypes(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := []cardset.Entry{
		{Name: "Undercover Skrull", Count: 14}, {Name: "Mind Rot", Count: 8}, {Name: "Doctor Doom", Count: 2}, {Name: "Claim the Kingdom", Count: 2},
		{Name: "Forest", Count: 7}, {Name: "Swamp", Count: 7},
	}
	lockstepRun(t, "creaturetypes", [2][]cardset.Entry{deck, deck}, 80, true, nil)
}
