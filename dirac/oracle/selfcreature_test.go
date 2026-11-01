package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Tom, Bert, and William come back as an artifact, and "when they die, if they were
// a creature" then does not fire, even with Necrosquito's noncreature dies line on
// the table (CR 700.4).
func TestDiesIfItWasACreature(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Tom, Bert, and William", Count: 8}, {Name: "Deadly Dispute", Count: 8}, {Name: "Doom Blade", Count: 6},
		{Name: "Necrosquito", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 6},
	}
	lockstepRun(t, "selfcreature", [2][]cardset.Entry{deck, deck}, 100, true, nil)
}
