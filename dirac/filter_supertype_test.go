package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLegendaryFilterBit: Creature.Legendary is the type AND the
// legendary supertype (gorge hasType), on the battlefield and off it.
func TestLegendaryFilterBit(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Baylen, the Haymaker", Count: 1}, {Name: "Downwind Ambusher", Count: 1}, {Name: "Plains", Count: 8}})
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TCreature, Flags2: kop.F2Legendary})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	leg, plain := sc.perm(id["Baylen, the Haymaker"], 0), sc.perm(id["Downwind Ambusher"], 0)
	if !sc.s.matchPerm(sc.k, f, sc.slot(leg), 0, NoSlot) {
		t.Fatal("a legendary creature did not match Creature.Legendary")
	}
	if sc.s.matchPerm(sc.k, f, sc.slot(plain), 0, NoSlot) {
		t.Fatal("a nonlegendary creature matched Creature.Legendary")
	}
	if !sc.s.matchCard(sc.k, f, id["Baylen, the Haymaker"], 0, 0) || sc.s.matchCard(sc.k, f, id["Downwind Ambusher"], 0, 0) {
		t.Fatal("the card matcher does not read F2Legendary")
	}
}
