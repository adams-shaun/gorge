package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestFilterBesideForagePaysFlash: Mouse Trapper's {2}{W} is payable from two
// Forests, Thornvault Forager's {T}: Add {G} and Hidden Grotto's filter ({1},
// {T}: Add one mana of any colour): a Forest funds the filter's {1}, the filter
// makes {W}, the other Forest and the Forager make {G}{G} (CR 605.3a, 601.2g).
// The Forager's forage ability is a Combo Any gap beside the filter; its forage
// is unpayable (empty graveyard) so the play rides the filter alone.
func TestFilterBesideForagePaysFlash(t *testing.T) {
	for _, gy := range []int{0, 3} {
		t.Run(fmt.Sprintf("graveyard%d", gy), func(t *testing.T) { filterBesideForage(t, gy) })
	}
}

func filterBesideForage(t *testing.T, gy int) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Mouse Trapper", Count: 1}, {Name: "Hidden Grotto", Count: 1},
		{Name: "Thornvault Forager", Count: 1}, {Name: "Forest", Count: 8}, {Name: "Plains", Count: 8}})
	sc.perm(id["Forest"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Hidden Grotto"], 0)
	sc.perm(id[forageManaName], 0)
	sc.grave(id["Forest"], 0, gy)
	mt := sc.hand(id["Mouse Trapper"], 0)
	if r := sc.s.StallReason(); r != StallNone {
		t.Fatalf("stalled: %s", StallName(r))
	}
	if !sc.castOffered(0, mt) {
		t.Fatal("Mouse Trapper's {2}{W} not offered over Hidden Grotto's filter beside the Forager")
	}
}
