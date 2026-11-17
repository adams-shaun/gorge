package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// CR 702.35b with 601.2g and 733.1: a madness cast begun mid-resolution
// carries no payment witness, so its mana window walks gorge's ask by ask. A
// Value Town ({T}: Add {U} or {R}) under Urborg, Tomb of Yawgmoth has two
// mana abilities (its printed one and the intrinsic Swamp's {T}: Add {B}); a
// doomed cost ({2}{B} with that one source untapped) taps it, fails, and the
// reversal (CR 733.1) keeps the mana it floated. gorge's ability ask lists the
// symbol-less {B} first but its answer takes the lowest symbolled option ({U}
// of the printed Combo); the kernel answered the folded view's owed pip {B}.
func TestMBCMadnessDoomedWindowLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Gisa's Bidding", Count: 8}, {Name: "Wheel and Deal", Count: 6}, {Name: "Value Town", Count: 14},
		{Name: "Urborg, Tomb of Yawgmoth", Count: 4}, {Name: "Swamp", Count: 8},
	}
	b := []cardset.Entry{
		{Name: "Wheel and Deal", Count: 10}, {Name: "Island", Count: 30},
	}
	fixtureLockstep(t, "tail-32", [2][]cardset.Entry{a, b}, 300)
}
