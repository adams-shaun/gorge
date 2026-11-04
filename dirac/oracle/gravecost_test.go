package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A graveyard activation is offered only with its whole cost payable: Gollum the
// Abandoned milled onto a board with nothing to sacrifice stays there.
func TestGraveAbilitySacPartLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := []cardset.Entry{
		{Name: "Gollum the Abandoned", Count: 12}, {Name: "Thought Scour", Count: 12},
		{Name: "Swamp", Count: 8}, {Name: "Island", Count: 8},
	}
	lockstepRun(t, "gravecost", [2][]cardset.Entry{deck, deck}, 80, true, nil)
}
