package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Dusk Mangler's three-way AlternateAdditionalCost (CR 601.2h): "sacrifice
// a creature, discard a card, or pay 4 life". The caster picks ONE part
// (kernel cast.go altAddOptions, gorge altAddAsk); an unpayable part
// reverses the cast (CR 733.1), and the ETB then makes each opponent
// sacrifice a creature (none here), discard a card and lose 4 life. The
// sacrifice part is unpayable in this fixture (no creature on the
// battlefield while the Mangler is on the stack), so the ask offers the
// discard and the life parts first, the sacrifice last.
func TestAltAddThreeWayLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{{Name: "Dusk Mangler", Count: 4}, {Name: "Swamp", Count: 36}}
	b := []cardset.Entry{{Name: "Swamp", Count: 40}}
	fixtureLockstep(t, "dr-mbc-aabb", [2][]cardset.Entry{a, b}, 60)
}
