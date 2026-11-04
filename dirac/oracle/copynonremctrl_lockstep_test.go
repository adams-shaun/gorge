package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Fractured Identity exiles target nonland permanent and gives each player
// other than its controller a token copy of it: two-player, the opponent of
// its last-known controller (Controller$ NonRememberedController over the
// RememberLKI$ exile), whoever cast it. Both decks cast it, so it takes
// permanents of either side, Howling Mine and Mind Stone among them.
func TestCopyNonRememberedControllerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Fractured Identity", Count: 8}, {Name: "Exocomp", Count: 6},
		{Name: "Gorn Captain", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Plains", Count: 9}, {Name: "Island", Count: 9},
	}
	b := []cardset.Entry{
		{Name: "Fractured Identity", Count: 4}, {Name: "Ceti Eel", Count: 6},
		{Name: "Howling Mine", Count: 2}, {Name: "Mind Stone", Count: 4},
		{Name: "Exocomp", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Island", Count: 10},
	}
	fixtureLockstep(t, "std-trc", [2][]cardset.Entry{a, b}, 300)
}
