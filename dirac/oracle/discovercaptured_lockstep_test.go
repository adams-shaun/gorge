package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Data, Exploring Humanity discovers 4 when its controller draws their second
// card each turn. Brainstorm's first draw is often that card, and Brainstorm
// can put it back on top, where the discover finds it again: back in the
// library it is a new object (CR 400.7), so the discover may cast it free
// (CR 701.57a) although its draw is what the trigger captured.
func TestDiscoverCapturedFoundLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Data, Exploring Humanity", Count: 6}, {Name: "Brainstorm", Count: 10},
		{Name: "Harmonize", Count: 6}, {Name: "Island", Count: 10}, {Name: "Forest", Count: 8},
	}
	b := []cardset.Entry{
		{Name: "Sickbay Orderly", Count: 10}, {Name: "Plains", Count: 30},
	}
	fixtureLockstep(t, "std-trc", [2][]cardset.Entry{a, b}, 300)
}
