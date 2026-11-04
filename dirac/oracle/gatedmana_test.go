package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A sacrifice in an activation's cost is chosen after its mana is tapped, so the
// colour a gated land made shows in the pool: gorge's plan takes the land's first
// ability, not the smallest colour vector of the pair.
func TestGatedLandFirstAbilityLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := []cardset.Entry{
		{Name: "Ripchain Razorkin", Count: 14}, {Name: "Thornspire Verge", Count: 16},
		{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
	}
	lockstepRun(t, "gatedsac", [2][]cardset.Entry{deck, deck}, 60, true, nil)
}
