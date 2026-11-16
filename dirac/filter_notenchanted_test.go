package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPredNotEnchanted: PredEnchanted with PredN 1 ("!enchanted") matches a
// permanent with no Aura attached, and a card off the battlefield.
func TestPredNotEnchanted(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Feather of Flight", Count: 1}, {Name: "Plains", Count: 8}})
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Pred: kop.PredEnchanted, PredN: 1})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	a, b := sc.perm(id["Valley Floodcaller"], 0), sc.perm(id["Valley Floodcaller"], 0)
	aura := sc.perm(id["Feather of Flight"], 0)
	sc.s.perms[sc.slot(aura)].attached = sc.slot(a)
	if sc.s.matchPerm(sc.k, f, sc.slot(a), 0, NoSlot) || !sc.s.matchPerm(sc.k, f, sc.slot(b), 0, NoSlot) {
		t.Fatal("!enchanted does not read the attached Aura")
	}
	if !sc.s.matchCard(sc.k, f, id["Valley Floodcaller"], 0, 0) {
		t.Fatal("a card off the battlefield is not unenchanted")
	}
}
