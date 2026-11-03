package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Tifa's Limit Break's Final Heaven tier adds twice the target's current power
// and toughness (gorge statAddMultiplier), read at resolution.
func TestPumpTripleLockstep(t *testing.T) {
	a := []cardset.Entry{{Name: "Tifa's Limit Break", Count: 10}, {Name: "Sazh's Chocobo", Count: 12}, {Name: "Forest", Count: 18}}
	b := []cardset.Entry{{Name: "Tifa's Limit Break", Count: 6}, {Name: "Sazh's Chocobo", Count: 10}, {Name: "Forest", Count: 14}, {Name: "Mountain", Count: 10}}
	fixtureLockstep(t, "std-fin", [2][]cardset.Entry{a, b}, 300)
}
