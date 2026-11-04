package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Replicator's activation creates a tapped token copy of a token (TokenTapped$
// True): the copy enters tapped, so its own entry triggers and a Food or
// Treasure copy's {T} cost see it tapped. Gwyndala, the Progeny's Neuroflux
// Heirloom and the Treasures of Savvy Profiteer give it Equipment, Food and
// Treasure tokens to copy.
func TestCopyTokenTappedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Replicator", Count: 6}, {Name: "Gwyndala, the Progeny", Count: 6},
		{Name: "Tricorder", Count: 3}, {Name: "Type-2 Phaser", Count: 3},
		{Name: "Savvy Profiteer", Count: 3},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 9},
	}
	b := []cardset.Entry{
		{Name: "Replicator", Count: 5}, {Name: "Gwyndala, the Progeny", Count: 5},
		{Name: "Academy Instructor", Count: 4}, {Name: "Savvy Profiteer", Count: 4},
		{Name: "Bat'leth", Count: 2},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	}
	fixtureLockstep(t, "std-trk", [2][]cardset.Entry{a, b}, 300)
}
