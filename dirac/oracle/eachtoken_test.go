package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// "Each player creates a token" mints for both seats, seat 0 first whoever
// controls the effect. Both decks hold the cards so both seats play them.
func TestEachPlayerTokenLockstep(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Party Dude", Count: 6}, {Name: "Edge Rover", Count: 6}, {Name: "Marching Duodrone", Count: 6},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 18},
	}
	lockstepRun(t, "eachtoken", [2][]cardset.Entry{deck, deck}, 40, true, nil)
}
