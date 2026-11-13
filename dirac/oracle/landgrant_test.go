package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Thranduil's Company grants its extra land only beside another Elf.
func TestConditionalLandGrantLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := []cardset.Entry{
		{Name: "Thranduil's Company", Count: 12}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Forest", Count: 14}, {Name: "Island", Count: 10},
	}
	lockstepRun(t, "landgrant", [2][]cardset.Entry{deck, deck}, 60, true, nil)
}
