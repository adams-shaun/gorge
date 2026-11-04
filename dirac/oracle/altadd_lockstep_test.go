package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Bitter Triumph's additional cost is a discard or 3 life (CR 601.2b, 601.2f).
// gorge offers both parts, the payable one first; a part that can't be paid
// reverses the cast (CR 733.1). With Forsaken Monument's {C} rider the mana
// window's census is incomplete, so a part short only on mana keeps the
// target menu open and the cast reverses in the window. A discard with no
// other card in hand is no mana: no window can pay it, gorge's target menu
// withholds every candidate, and the cast reverses as the part is chosen.
func TestAltAddUnpayableDiscardLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Bitter Triumph", Count: 10}, {Name: "Forsaken Monument", Count: 4},
		{Name: "Mind Stone", Count: 4}, {Name: "Sol Ring", Count: 2},
		{Name: "Swamp", Count: 20},
	}
	b := []cardset.Entry{
		{Name: "Sickbay Orderly", Count: 10}, {Name: "Operations Officer", Count: 6},
		{Name: "Plains", Count: 24},
	}
	fixtureLockstep(t, "std-trc", [2][]cardset.Entry{a, b}, 300)
}
