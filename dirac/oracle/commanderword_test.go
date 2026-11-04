package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Collective Override, Janice Rand, Queen's Diamond and Command Beacon read
// the IsCommander filter word (kop.W7IsCommander). No seat has a commander in
// a game either engine plays here, so the charm offers one mode, the commander
// triggers never fire, Queen's Diamond's mana is unspendable and Command
// Beacon's activation sacrifices the land and moves nothing.
func TestCommanderWordLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Collective Override", Count: 5}, {Name: "Janice Rand, Essential Aide", Count: 4},
		{Name: "Queen's Diamond", Count: 4}, {Name: "Command Beacon", Count: 5},
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 7}, {Name: "Forest", Count: 7},
	}
	b := []cardset.Entry{
		{Name: "Collective Override", Count: 3}, {Name: "Janice Rand, Essential Aide", Count: 5},
		{Name: "Queen's Diamond", Count: 3}, {Name: "Command Beacon", Count: 3},
		{Name: "Exocomp", Count: 6}, {Name: "Gorn Captain", Count: 4},
		{Name: "Plains", Count: 6}, {Name: "Island", Count: 5}, {Name: "Forest", Count: 5},
	}
	fixtureLockstep(t, "std-trc", [2][]cardset.Entry{a, b}, 300)
}
